package auxservice

// TODO:

import (
	"context"
	"log"
	"os"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/ramrod-project/aux-services-service/helper"

	"github.com/docker/docker/api/types"
)

// SignalCatcher takes a channel of os.Signals from the main
// function and handles SIGTERM, SIGKILL, and SIGINT
func SignalCatcher(sigc <-chan os.Signal, sigCancel context.CancelFunc, id string) <-chan struct{} {
	signaller := make(chan struct{})

	go func() {
		timeout := 3 * time.Second

		ctx, cancel := context.WithCancel(context.Background())
		dockerClient, err := client.NewEnvClient()
		if err != nil {
			panic(err)
		}

		s := <-sigc
		sigCancel()
		switch s {
		case syscall.SIGTERM:
			log.Printf("handle SIGTERM")
			err := dockerClient.ContainerStop(ctx, id, &timeout)
			if err != nil {
				log.Printf("%v", err)
			}
		case syscall.SIGKILL:
			log.Printf("handle SIGKILL")
			err := dockerClient.ContainerKill(ctx, id, "SIGKILL")
			if err != nil {
				log.Printf("%v", err)
			}
		case syscall.SIGINT:
			log.Printf("handle SIGINT")
			err := dockerClient.ContainerStop(ctx, id, &timeout)
			if err != nil {
				log.Printf("%v", err)
			}
		}
		log.Printf("Sending service kill signal")
		signaller <- struct{}{}
		cancel()
		return
	}()

	return signaller
}

// MonitorAux monitors container events of the
// aux services container. If the container
// dies unexpectedly, the service should also
// die (so Docker automatically restarts it)
func MonitorAux(ctx context.Context) (<-chan struct{}, <-chan error) {
	signaller := make(chan struct{})

	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	events, errs := dockerClient.Events(
		ctx,
		types.EventsOptions{
			Filters: helper.GetContainerFilterArgs(helper.AuxContainerName),
		},
	)

	go func() {
	L:
		for {
			select {
			case e := <-events:
				switch e.Status {
				case "create":
					log.Printf("Aux services created")
				case "start":
					log.Printf("Aux services started")
				case "die":
					log.Printf("Container dead, dying...")
					break L
				}
			case <-ctx.Done():
				return
			}
		}
		signaller <- struct{}{}
		return
	}()
	return signaller, errs
}

// CheckForAux checks to see if the aux services
// container is already running.
func CheckForAux(ctx context.Context) string {
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	containers, err := dockerClient.ContainerList(
		ctx,
		types.ContainerListOptions{},
	)
	if err != nil {
		panic(err)
	}

	for _, c := range containers {
		for _, n := range c.Names {
			if n == helper.AuxContainerName {
				return c.ID
			}
		}
	}
	return ""
}
