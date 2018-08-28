package auxservice

import (
	"context"
	"errors"
	"log"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/ramrod-project/aux-services-service/helper"
	"github.com/stretchr/testify/assert"
)

func TestCheckForAux(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	defer cancel()

	netRes, err := dockerClient.NetworkCreate(ctx, "pcp", types.NetworkCreate{
		Driver:     "overlay",
		Attachable: true,
	})
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	var conid string

	type args struct {
		c context.Context
	}
	tests := []struct {
		name  string
		start bool
		args  args
		want  string
	}{
		{
			name:  "isnt there",
			start: false,
			args: args{
				c: ctx,
			},
			want: "",
		},
		{
			name:  "is there",
			start: false,
			args: args{
				c: ctx,
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				con, err := dockerClient.ContainerCreate(
					ctx,
					&helper.AuxContainerConfig,
					&helper.AuxHostConfig,
					&helper.AuxNetConfig,
					helper.AuxContainerName,
				)
				conid = con.ID
				tt.want = conid
				if err != nil {
					log.Printf("Container create error")
					t.Errorf("%v", err)
					return
				}
				err = dockerClient.ContainerStart(ctx, con.ID, types.ContainerStartOptions{})
				if err != nil {
					t.Errorf("%v", err)
					return
				}
			}
			got := CheckForAux(ctx)
			assert.Equal(t, tt.want, got)
		})
	}
	helper.KillAux(ctx, dockerClient, conid)
	err = helper.KillNet(netRes.ID)
	if err != nil {
		t.Errorf("%v", err)
	}
}

func TestMonitorAux(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	dockerClient, err := client.NewEnvClient()
	defer cancel()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	netRes, err := dockerClient.NetworkCreate(ctx, "pcp", types.NetworkCreate{
		Driver:     "overlay",
		Attachable: true,
	})
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	// aux services container
	con, err := dockerClient.ContainerCreate(
		ctx,
		&helper.AuxContainerConfig,
		&helper.AuxHostConfig,
		&helper.AuxNetConfig,
		helper.AuxContainerName,
	)
	if err != nil {
		log.Printf("Container create error")
		t.Errorf("%v", err)
		return
	}
	err = dockerClient.ContainerStart(ctx, con.ID, types.ContainerStartOptions{})
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	// fake dumb container
	conFake, err := dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Hostname: "testcontainer",
			Cmd:      []string{"sleep", "300"},
			Image:    "alpine:3.7",
		},
		&container.HostConfig{
			NetworkMode: "default",
			AutoRemove:  true,
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				"pcp": &network.EndpointSettings{
					Aliases:   []string{"testcontainer"},
					NetworkID: helper.Net.ID,
				},
			},
		},
		"testcontainer",
	)
	if err != nil {
		log.Printf("Test container create error")
		t.Errorf("%v", err)
		return
	}
	err = dockerClient.ContainerStart(ctx, conFake.ID, types.ContainerStartOptions{})
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	start := time.Now()
	found := false
	for time.Since(start) < 15*time.Second {
		c, err := dockerClient.ContainerInspect(ctx, con.ID)
		if err == nil {
			if c.State.Status == "running" {
				found = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !found {
		t.Errorf("container not started in timeout")
		return
	}

	tests := []struct {
		name    string
		killCon string
		wantErr bool
		err     error
		wait    func(chan bool, ...interface{})
	}{
		{
			name:    "cancel context",
			wantErr: true,
			wait: func(res chan bool, args ...interface{}) {
				timeoutCtx, cancelTimeout := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancelTimeout()

				err := args[1].(<-chan error)
				cancel := args[0].(context.CancelFunc)
				cancel()

				select {
				case <-timeoutCtx.Done():
					close(res)
					return
				case e := <-err:
					assert.Equal(t, errors.New("context canceled"), e)
					res <- true
				}
			},
		},
		{
			name:    "test container dead (timeout)",
			killCon: conFake.ID,
			wait: func(res chan bool, args ...interface{}) {
				timeoutCtx, cancelTimeout := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancelTimeout()

				id := args[0].(string)
				sig := args[1].(<-chan struct{})

				err := dockerClient.ContainerKill(timeoutCtx, id, "SIGKILL")
				if err != nil {
					t.Errorf("%v", err)
					close(res)
					return
				}

				select {
				case <-timeoutCtx.Done():
					res <- true
					close(res)
					return
				case <-sig:
					close(res)
					return
				}
			},
		},
		{
			name:    "aux container dead",
			killCon: con.ID,
			wait: func(res chan bool, args ...interface{}) {
				timeoutCtx, cancelTimeout := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancelTimeout()

				id := args[0].(string)
				sig := args[1].(<-chan struct{})

				err := dockerClient.ContainerKill(timeoutCtx, id, "SIGKILL")
				if err != nil {
					t.Errorf("%v", err)
					close(res)
					return
				}

				select {
				case <-timeoutCtx.Done():
					close(res)
					return
				case <-sig:
					res <- true
					close(res)
					return
				}
			},
		},
		// add test case for wrong container dying
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// overall timeout for test
			testTimeout, testCancel := context.WithTimeout(context.Background(), 10*time.Second)
			// we will manually cancel this context, so it must be separate from
			// the overall test context
			newCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			sig, err := MonitorAux(newCtx)
			defer testCancel()
			defer cancel()

			var resChan <-chan bool
			if tt.wantErr {
				resChan = func() <-chan bool {
					res := make(chan bool)
					go tt.wait(res, cancel, err)
					return res
				}()
			} else {
				resChan = func() <-chan bool {
					res := make(chan bool)
					go tt.wait(res, tt.killCon, sig)
					return res
				}()
			}
			select {
			case <-testTimeout.Done():
				t.Errorf("test context timed out")
			case v := <-resChan:
				assert.True(t, v)
			}
		})
	}

	helper.KillAux(ctx, dockerClient, con.ID)
	helper.KillAux(ctx, dockerClient, conFake.ID)
	err = helper.KillNet(netRes.ID)
	if err != nil {
		t.Errorf("%v", err)
	}
}

func TestSignalCatcher(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	dockerClient, err := client.NewEnvClient()
	defer cancel()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	netRes, err := dockerClient.NetworkCreate(ctx, "pcp", types.NetworkCreate{
		Driver:     "overlay",
		Attachable: true,
	})
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	tests := []struct {
		name string
		sig  syscall.Signal
	}{
		{
			name: "SIGTERM",
			sig:  syscall.SIGTERM,
		},
		{
			name: "SIGKILL",
			sig:  syscall.SIGKILL,
		},
		{
			name: "SIGINT",
			sig:  syscall.SIGINT,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			con, err := helper.StartAux(ctx, dockerClient)
			if err != nil {
				t.Errorf("%v", err)
			}

			testCtx, testCancel := context.WithCancel(context.Background())
			defer testCancel()

			sigSend := make(chan os.Signal)
			sigRecv := SignalCatcher(sigSend, testCancel, con.ID)

			timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer timeoutCancel()

			sigSend <- tt.sig
			select {
			case <-timeoutCtx.Done():
				t.Errorf("should have received signal")
			case s := <-sigRecv:
				assert.IsType(t, struct{}{}, s)
			}

			select {
			case <-testCtx.Done():
				assert.True(t, true)
			case <-timeoutCtx.Done():
				t.Errorf("should have cancelled test context")
			}

			helper.KillAux(ctx, dockerClient, con.ID)
		})
	}
	err = helper.KillNet(netRes.ID)
	if err != nil {
		t.Errorf("%v", err)
	}
}
