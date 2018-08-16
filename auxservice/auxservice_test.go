package auxservice

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
)

func Test_concatString(t *testing.T) {
	type args struct {
		k   string
		sep string
		v   string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test string 1",
			args: args{
				k:   "me",
				sep: "=",
				v:   "awesome",
			},
			want: "me=awesome",
		},
		{
			name: "test string 2",
			args: args{
				k:   "you",
				sep: ":",
				v:   "lame",
			},
			want: "you:lame",
		},
		{
			name: "test string 3",
			args: args{
				k:   "sfrgjsrtusrykm",
				sep: "-",
				v:   "ryjsr7yia6iua6i",
			},
			want: "sfrgjsrtusrykm-ryjsr7yia6iua6i",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := concatString(tt.args.k, tt.args.sep, tt.args.v); got != tt.want {
				t.Errorf("concatString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getEnvByKey(t *testing.T) {
	tests := []struct {
		name string
		k    string
		want string
	}{
		{
			name: "Test env 1",
			k:    "TEST1",
			want: "TEST1=TEST1",
		},
		{
			name: "Test env 2",
			k:    "TEST2",
			want: "TEST2=ghjstdgjhafhgjnysthy",
		},
		{
			name: "Test empty",
			k:    "ESAODUGHUQEWTR",
			want: "ESAODUGHUQEWTR=",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(tt.k, strings.Split(tt.want, "=")[1])
			res := getEnvByKey(tt.k)
			assert.Equal(t, tt.want, res)
			os.Setenv(tt.k, "")
		})
	}
}

func Test_getTagFromEnv(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "Dev",
			want: "dev",
		},
		{
			name: "Qa",
			want: "qa",
		},
		{
			name: "Latest",
			want: "latest",
		},
		{
			name: "None",
			want: "latest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := os.Getenv("TAG")
			if tt.name == "None" {
				os.Setenv("TAG", "")
			} else {
				os.Setenv("TAG", tt.want)
			}
			assert.Equal(t, tt.want, getTagFromEnv())
			os.Setenv("TAG", old)
		})
	}
}

func Test_getPorts(t *testing.T) {
	tests := []struct {
		name string
		in   [][]string
		want []nat.Port
	}{
		{
			name: "Normal tcp",
			in: [][]string{
				[]string{"tcp", "1000"},
				[]string{"tcp", "2000"},
				[]string{"tcp", "3000"},
			},
			want: []nat.Port{
				"1000/tcp",
				"2000/tcp",
				"3000/tcp",
			},
		},
		{
			name: "Normal udp",
			in: [][]string{
				[]string{"udp", "1000"},
				[]string{"udp", "2000"},
				[]string{"udp", "3000"},
			},
			want: []nat.Port{
				"1000/udp",
				"2000/udp",
				"3000/udp",
			},
		},
		{
			name: "Normal mix",
			in: [][]string{
				[]string{"udp", "1000"},
				[]string{"tcp", "2000"},
				[]string{"udp", "3000"},
			},
			want: []nat.Port{
				"1000/udp",
				"2000/tcp",
				"3000/udp",
			},
		},
		{
			name: "Bad port",
			in: [][]string{
				[]string{"1000", "blah"},
			},
			want: []nat.Port{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPorts(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPorts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getPortSet(t *testing.T) {
	tests := []struct {
		name string
		want nat.PortSet
	}{
		{
			name: "Normal 1",
			want: nat.PortSet{
				"20/tcp": struct{}{},
				"21/tcp": struct{}{},
				"80/tcp": struct{}{},
				"53/udp": struct{}{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPortSet(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPortSet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getIP(t *testing.T) {

	var (
		ips []string
	)

	// get local interfaces from node
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			t.Errorf("%v", err)
			return
		}
		for _, addr := range addrs {
			ips = append(ips, strings.Split(addr.String(), "/")[0])
		}
	}

	tests := []struct {
		name string
		want []string
	}{
		{
			name: "Get IP",
			want: ips,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getIP()
			found := false
			for _, ip := range tt.want {
				if ip == got {
					found = true
					break
				}
			}
			assert.True(t, found)
		})
	}
}

func Test_getPortMap(t *testing.T) {
	type args struct {
		ip    string
		ports []nat.Port
	}
	tests := []struct {
		name string
		args args
		want nat.PortMap
	}{
		{
			name: "Normal 1",
			args: args{
				ip: "192.168.1.1",
				ports: []nat.Port{
					"1000/tcp",
				},
			},
			want: nat.PortMap{
				"1000/tcp": []nat.PortBinding{
					nat.PortBinding{
						HostIP:   "192.168.1.1",
						HostPort: "1000",
					},
				},
			},
		},
		{
			name: "Normal 2",
			args: args{
				ip: "192.168.1.2",
				ports: []nat.Port{
					"999/udp",
				},
			},
			want: nat.PortMap{
				"999/udp": []nat.PortBinding{
					nat.PortBinding{
						HostIP:   "192.168.1.2",
						HostPort: "999",
					},
				},
			},
		},
		{
			name: "Complex",
			args: args{
				ip: "10.0.0.4",
				ports: []nat.Port{
					"999/udp",
					"1000/tcp",
					"1001/udp",
					"1002/tcp",
					"666/udp",
				},
			},
			want: nat.PortMap{
				"999/udp": []nat.PortBinding{
					nat.PortBinding{
						HostIP:   "10.0.0.4",
						HostPort: "999",
					},
				},
				"1000/tcp": []nat.PortBinding{
					nat.PortBinding{
						HostIP:   "10.0.0.4",
						HostPort: "1000",
					},
				},
				"1001/udp": []nat.PortBinding{
					nat.PortBinding{
						HostIP:   "10.0.0.4",
						HostPort: "1001",
					},
				},
				"1002/tcp": []nat.PortBinding{
					nat.PortBinding{
						HostIP:   "10.0.0.4",
						HostPort: "1002",
					},
				},
				"666/udp": []nat.PortBinding{
					nat.PortBinding{
						HostIP:   "10.0.0.4",
						HostPort: "666",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPortMap(tt.args.ip, tt.args.ports); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPortMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getArgs(t *testing.T) {

	f := filters.NewArgs()
	f.Add(
		"Type", "container",
	)
	f.Add(
		"Actor.Attributes.name", AuxContainerName,
	)
	tests := []struct {
		name string
		want filters.Args
	}{
		{
			name: "Normal 1",
			want: f,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getArgs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func StartAux(ctx context.Context, dockerClient *client.Client) (container.ContainerCreateCreatedBody, error) {
	con, err := dockerClient.ContainerCreate(
		ctx,
		&AuxContainerConfig,
		&AuxHostConfig,
		&AuxNetConfig,
		AuxContainerName,
	)
	if err != nil {
		log.Printf("Container create error")
		return container.ContainerCreateCreatedBody{}, err
	}
	err = dockerClient.ContainerStart(ctx, con.ID, types.ContainerStartOptions{})
	if err != nil {
		return container.ContainerCreateCreatedBody{}, err
	}
	return con, nil
}

func KillAux(ctx context.Context, dockerClient *client.Client, id string) error {
	start := time.Now()
	var err error
	for time.Since(start) < 10*time.Second {
		err = dockerClient.ContainerKill(ctx, id, "SIGKILL")
		if err == nil {
			return nil
		}
	}
	return err
}

func KillNet(netid string) error {
	ctx, cancel := context.WithCancel(context.Background())
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	defer cancel()

	start := time.Now()
	for time.Since(start) < 10*time.Second {
		dockerClient.NetworkRemove(ctx, netid)
		time.Sleep(time.Second)
		_, err := dockerClient.NetworkInspect(ctx, netid)
		if err != nil {
			_, err := dockerClient.NetworksPrune(ctx, filters.Args{})
			if err == nil {
				return nil
			}
		}
	}
	return errors.New("couldn't clean up networks")
}

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
					&AuxContainerConfig,
					&AuxHostConfig,
					&AuxNetConfig,
					AuxContainerName,
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
	KillAux(ctx, dockerClient, conid)
	err = KillNet(netRes.ID)
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

	con, err := dockerClient.ContainerCreate(
		ctx,
		&AuxContainerConfig,
		&AuxHostConfig,
		&AuxNetConfig,
		AuxContainerName,
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
		res     struct{}
		wantErr bool
		err     error
	}{
		{
			name:    "cancel context",
			wantErr: true,
			err:     errors.New("context canceled"),
		},
		{
			name: "container dead",
			res:  struct{}{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newCtx, cancel := context.WithCancel(context.Background())
			sig, err := MonitorAux(newCtx)

			timeoutCtx, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelTimeout()

			if tt.wantErr {
				cancel()
				select {
				case <-timeoutCtx.Done():
					t.Errorf("err %v not received", tt.err)
					assert.True(t, false)
					return
				case e := <-err:
					assert.Equal(t, tt.err, e)
				}
			} else {
				defer cancel()

				err := dockerClient.ContainerKill(ctx, con.ID, "SIGKILL")
				if err != nil {
					t.Errorf("error killing container: %v", err)
					return
				}

				select {
				case <-timeoutCtx.Done():
					t.Errorf("signal not received")
					assert.True(t, false)
					return
				case r := <-sig:
					assert.Equal(t, tt.res, r)
				}
			}
		})
	}

	KillAux(ctx, dockerClient, con.ID)
	err = KillNet(netRes.ID)
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
			con, err := StartAux(ctx, dockerClient)
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

			KillAux(ctx, dockerClient, con.ID)
		})
	}
	err = KillNet(netRes.ID)
	if err != nil {
		t.Errorf("%v", err)
	}
}
