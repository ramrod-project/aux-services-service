package auxservice

import (
	"bytes"
	"context"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/docker/docker/client"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"

	"github.com/docker/go-connections/nat"
)

var Net = func() types.NetworkResource {
	ctx, cancel := context.WithCancel(context.Background())
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	defer cancel()

	netFilter := filters.NewArgs()
	netFilter.Add("name", "pcp")

	nets, err := dockerClient.NetworkList(ctx, types.NetworkListOptions{
		Filters: netFilter,
	})
	if err != nil || len(nets) != 1 {
		panic(err)
	}
	return nets[0]
}()

var defaultEnvs = map[string]string{
	"LOGLEVEL": "DEBUG",
	"STAGE":    "DEV",
}

func concatString(k string, sep string, v string) string {
	var stringBuf bytes.Buffer

	stringBuf.WriteString(k)
	stringBuf.WriteString(sep)
	stringBuf.WriteString(v)

	return stringBuf.String()
}

func getEnvByKey(k string) string {
	var env string
	env = os.Getenv(k)

	if env == "" {
		env = defaultEnvs[k]
	}

	return concatString(k, "=", env)
}

func getTagFromEnv() string {
	temp := os.Getenv("TAG")
	if temp == "" {
		temp = os.Getenv("TRAVIS_BRANCH")
	}
	if temp == "" {
		temp = "latest"
	}
	return temp
}

func getPorts(in [][]string) []nat.Port {
	ports := make([]nat.Port, len(in))
	for i, p := range in {
		port, err := nat.NewPort(p[0], p[1])
		if err != nil {
			panic(err)
		}
		ports[i] = port
	}
	return ports
}

func getPortSet() nat.PortSet {
	var portSet = make(nat.PortSet)
	for _, p := range getPorts([][]string{[]string{"tcp", "20"}, []string{"tcp", "21"}, []string{"tcp", "80"}, []string{"udp", "53"}}) {
		portSet[p] = struct{}{}
	}
	return portSet
}

func getIP() string {
	ctx, cancel := context.WithCancel(context.Background())
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	defer cancel()

	nodes, err := dockerClient.NodeList(ctx, types.NodeListOptions{})
	if err != nil {
		panic(err)
	}

	for _, n := range nodes {
		if n.ManagerStatus.Leader {
			return n.Status.Addr
		}
	}
	return ""
}

func getPortMap(ip string, ports []nat.Port) nat.PortMap {
	var portM = make(nat.PortMap)
	for _, p := range ports {
		portM[p] = []nat.PortBinding{
			nat.PortBinding{
				HostIP:   ip,
				HostPort: strings.Split(string(p), "/")[0],
			},
		}
	}
	return portM
}

func getArgs() filters.Args {
	args := filters.NewArgs()
	args.Add(
		"Type",
		"container",
	)
	args.Add(
		"Actor.Attributes.name",
		AuxContainerName,
	)
	return args
}

// AuxContainerConfig
var AuxContainerConfig = container.Config{
	Hostname: AuxContainerName,
	Env: []string{
		getEnvByKey("STAGE"),
		getEnvByKey("LOGLEVEL"),
	},
	ExposedPorts: getPortSet(),
	Image: concatString(
		"ramrodpcp/auxiliary-services",
		":",
		getTagFromEnv(),
	),
}

// AuxHostConfig
var AuxHostConfig = container.HostConfig{
	/*Binds: []string{
		"brain-volume:/www/files/brain",
	},*/
	NetworkMode: "default",
	PortBindings: getPortMap(
		getIP(),
		getPorts([][]string{
			[]string{"tcp", "20"},
			[]string{"tcp", "21"},
			[]string{"tcp", "80"},
			[]string{"udp", "53"},
		}),
	),
	RestartPolicy: container.RestartPolicy{
		MaximumRetryCount: 3,
	},
	AutoRemove:  true,
	CapAdd:      strslice.StrSlice{"SYS_ADMIN"},
	SecurityOpt: []string{"apparmor:unconfined"},
	Resources: container.Resources{
		Devices: []container.DeviceMapping{
			container.DeviceMapping{
				PathOnHost:        "/dev/fuse",
				PathInContainer:   "/dev/fuse",
				CgroupPermissions: "mrw",
			},
		},
	},
}

// AuxNetConfig
var AuxNetConfig = network.NetworkingConfig{
	EndpointsConfig: map[string]*network.EndpointSettings{
		"pcp": &network.EndpointSettings{
			Aliases:   []string{"auxiliary-services"},
			NetworkID: Net.ID,
		},
	},
}

// AuxName
var AuxContainerName = "aux-services"

// SignalCatcher
func SignalCatcher(sigc chan os.Signal) {
	s := <-sigc
	switch s {
	case syscall.SIGTERM:
		log.Printf("handle SIGTERM")
	case syscall.SIGKILL:
		log.Printf("handle SIGKILL")
	case syscall.SIGINT:
		log.Printf("handle SIGINT")
	default:
		log.Printf("signal %v not handled", s)
	}
	close(sigc)
	os.Exit(0)
}

// MonitorAux function
func MonitorAux(ctx context.Context) <-chan error {
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	events, errs := dockerClient.Events(
		ctx,
		types.EventsOptions{
			Filters: getArgs(),
		},
	)
	go func() {
		for e := range events {
			switch e.Status {
			case "create":
				log.Printf("Aux services created")
			case "start":
				log.Printf("Aux services started")
			case "stop":
				log.Printf("Aux services stop")
			case "kill":
				log.Printf("Aux services kill")
			case "die":
				log.Printf("Aux services dead")
			}
		}
	}()
	return errs
}

// CheckForAux
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
			if n == AuxContainerName {
				return c.ID
			}
		}
	}
	return ""
}
