package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/ramrod-project/aux-services-service/auxservice"
)

func main() {
	// Config and start goroutine to handle sigterm/kill
	// On sigterm/kill, stop/kill the aux services container
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGINT)
	go auxservice.SignalCatcher(sigc)

	// Initialize context and docker client
	ctx, cancel := context.WithCancel(context.Background())
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	defer cancel()

	// Check if aux plugin is currently running
	//     if true, kill the container
	if id := auxservice.CheckForAux(ctx); id != "" {
		dockerClient.ContainerKill(ctx, id, "SIGKILL")
	}

	// Create the container with the provided configs
	con, err := dockerClient.ContainerCreate(
		ctx,
		&auxservice.AuxContainerConfig,
		&auxservice.AuxHostConfig,
		&auxservice.AuxNetConfig,
		auxservice.AuxContainerName,
	)
	if err != nil {
		log.Printf("Container create error")
		panic(err)
	}
	log.Printf("Container created...")

	// Start aux services container
	err = dockerClient.ContainerStart(ctx, con.ID, types.ContainerStartOptions{})
	if err != nil {
		log.Printf("Container start error")
		panic(err)
	}
	log.Printf("Container started...")

	// Monitor events with filter https://godoc.org/github.com/docker/docker/api/types/filters
	//     if the container dies, restart it
	errs := auxservice.MonitorAux(ctx)
	for e := range errs {
		log.Printf("Error from monitor")
		panic(e)
	}
}
