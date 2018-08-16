package test

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/ramrod-project/aux-services-service/auxservice"
)

func getImage(image string) string {
	var stringBuf bytes.Buffer

	tag := os.Getenv("TAG")
	if tag == "" {
		tag = "latest"
	}

	stringBuf.WriteString(image)
	stringBuf.WriteString(":")
	stringBuf.WriteString(tag)

	return stringBuf.String()
}

func getEnvStage() string {
	var stringBuf bytes.Buffer

	tag := os.Getenv("TAG")
	if tag == "" {
		tag = "latest"
	}

	stringBuf.WriteString("TAG=")
	stringBuf.WriteString(tag)

	return stringBuf.String()
}

var auxServiceSpec = swarm.ServiceSpec{
	Annotations: swarm.Annotations{
		Name: "AuxiliaryServices",
	},
	TaskTemplate: swarm.TaskSpec{
		ContainerSpec: swarm.ContainerSpec{
			DNSConfig: &swarm.DNSConfig{},
			Image:     getImage("ramrodpcp/auxiliary-wrapper"),
			Mounts: []mount.Mount{
				mount.Mount{
					Type:   mount.TypeBind,
					Source: "/var/run/docker.sock",
					Target: "/var/run/docker.sock",
				},
			},
			Env: []string{
				getEnvStage(),
			},
		},
		Networks: []swarm.NetworkAttachmentConfig{
			swarm.NetworkAttachmentConfig{
				Target: "pcp",
			},
		},
		RestartPolicy: &swarm.RestartPolicy{
			Condition: "on-failure",
		},
	},
	EndpointSpec: &swarm.EndpointSpec{
		Mode: swarm.ResolutionModeVIP,
	},
}

func timoutTester(ctx context.Context, args []interface{}, f func(args ...interface{}) bool) <-chan bool {
	done := make(chan bool)

	go func() {
		for {
			recv := make(chan bool)

			go func() {
				recv <- f(args...)
				close(recv)
				return
			}()

			select {
			case <-ctx.Done():
				done <- false
				close(done)
				return
			case b := <-recv:
				done <- b
				close(done)
				return
			}
		}
	}()

	return done
}

func StartAux(ctx context.Context, dockerClient *client.Client) (container.ContainerCreateCreatedBody, error) {
	con, err := dockerClient.ContainerCreate(
		ctx,
		&auxservice.AuxContainerConfig,
		&auxservice.AuxHostConfig,
		&auxservice.AuxNetConfig,
		auxservice.AuxContainerName,
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

func StartAuxService(ctx context.Context, dockerClient *client.Client, spec swarm.ServiceSpec) (string, error) {

	// Start service
	result, err := dockerClient.ServiceCreate(ctx, spec, types.ServiceCreateOptions{})
	if err != nil {
		return "", err
	}
	return result.ID, nil
}

// KillAux
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

// KillNet
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

func KillAuxService(ctx context.Context, dockerClient *client.Client, svcID string) {
	start := time.Now()
	for time.Since(start) < 10*time.Second {
		err := dockerClient.ServiceRemove(ctx, svcID)
		if err != nil {
			break
		}
		time.Sleep(time.Second)
	}
	for time.Since(start) < 15*time.Second {
		containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{})
		if err == nil {
			if len(containers) == 0 {
				break
			}
			for _, c := range containers {
				err = dockerClient.ContainerKill(ctx, c.ID, "")
				if err == nil {
					dockerClient.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true})
				}
			}
		}
		time.Sleep(time.Second)
	}
}
