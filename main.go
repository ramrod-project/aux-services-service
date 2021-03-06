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
	"github.com/ramrod-project/aux-services-service/helper"
)

func main() {
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
		&helper.AuxContainerConfig,
		&helper.AuxHostConfig,
		&helper.AuxNetConfig,
		helper.AuxContainerName,
	)
	if err != nil {
		log.Printf("Container create error")
		panic(err)
	}
	log.Printf("Container created...")

	// On sigterm/kill, stop/kill the aux services container
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGINT)

	// Shared context, cancelled when the service is removed
	// This prevents a race condition between the aux service
	// container monitor and the service monitor.
	sigContext, sigCancel := context.WithCancel(context.Background())

	// sigService channel will be populated if the service is
	// torn down by the docker daemon
	sigService := auxservice.SignalCatcher(sigc, sigCancel, con.ID)

	// Start aux services container
	err = dockerClient.ContainerStart(ctx, con.ID, types.ContainerStartOptions{})
	if err != nil {
		log.Printf("Container start error")
		panic(err)
	}
	log.Printf("Container started...")

	// sigAux channel will be populated if the aux services container
	// dies
	sigAux, errs := auxservice.MonitorAux(sigContext)

	go func(sigS <-chan struct{}, sigA <-chan struct{}) {
		select {
		case <-sigS:
			log.Printf("Service remove signal.")
			os.Exit(0)
		case <-sigA:
			log.Printf("Container dead, service dying...")
			os.Exit(1)
		}
	}(sigService, sigAux)

	// Monitor event errors
	go func() {
		for e := range errs {
			log.Printf("Error from monitor: %v", e)
		}
	}()

	// Monitors the signal channels.
	// and will cause the process to exit if either the
	// service is removed or the aux container dies
	// (Exit code 1 tells docker daemon to restart
	// the service).
	select {
	case <-sigService:
		log.Printf("Service remove signal.")
		os.Exit(0)
	case <-sigAux:
		log.Printf("Container dead, service dying...")
		os.Exit(1)
	}
}
