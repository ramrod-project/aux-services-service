package helper

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// DefaultPorts is the set of default
// ports the aux services container uses.
var DefaultPorts = [][]string{
	[]string{"tcp", "20"},
	[]string{"tcp", "21"},
	[]string{"tcp", "80"},
	[]string{"udp", "53"},
	[]string{"tcp", "10090"},
	[]string{"tcp", "10091"},
	[]string{"tcp", "10092"},
	[]string{"tcp", "10093"},
	[]string{"tcp", "10094"},
	[]string{"tcp", "10095"},
	[]string{"tcp", "10096"},
	[]string{"tcp", "10097"},
	[]string{"tcp", "10098"},
	[]string{"tcp", "10099"},
	[]string{"tcp", "10100"},
}

var defaultEnvs = map[string]string{
	"LOGLEVEL": "DEBUG",
	"STAGE":    "DEV",
}

// ConcatString returns a new string from two strings
// and a separator.
func ConcatString(k string, sep string, v string) string {
	var stringBuf bytes.Buffer

	stringBuf.WriteString(k)
	stringBuf.WriteString(sep)
	stringBuf.WriteString(v)

	return stringBuf.String()
}

// GetEnvByKey gets environment variables
// by key and returns them in the form
// KEY=VALUE. It tries to pull from the
// default envs if there is nothing set.
func GetEnvByKey(k string) string {
	var env string
	env = os.Getenv(k)

	if env == "" {
		env = defaultEnvs[k]
	}

	return ConcatString(k, "=", env)
}

// GetTagFromEnv gets the Docker tag
// (dev, qa, latest) from the environment
// variable TAG.
func GetTagFromEnv() string {
	temp := os.Getenv("TAG")
	if temp == "" {
		temp = "latest"
	}
	return temp
}

// GetPorts returns a []nat.Port from
// a slice of string slices in the format
// ["proto", "port"].
func GetPorts(in [][]string) []nat.Port {
	ports := make([]nat.Port, len(in))
	for i, p := range in {
		port, err := nat.NewPort(p[0], p[1])
		if err != nil {
			log.Printf("%v", err)
			return []nat.Port{}
		}
		ports[i] = port
	}
	return ports
}

// GetPortSet creates the default port
// set for the auxiliary services container
func GetPortSet(ports [][]string) nat.PortSet {
	var portSet = make(nat.PortSet)
	for _, p := range GetPorts(ports) {
		portSet[p] = struct{}{}
	}
	return portSet
}

// GetIP gets the IP address of the
// swarm manager
func GetIP() string {
	ctx, cancel := context.WithCancel(context.Background())
	dockerClient, err := client.NewEnvClient()
	defer cancel()
	if err != nil {
		panic(err)
	}

	nodes, err := dockerClient.NodeList(ctx, types.NodeListOptions{})
	if err != nil {
		panic(err)
	}

	for _, n := range nodes {
		if n.Spec.Role == swarm.NodeRoleWorker {
			continue
		}
		if n.ManagerStatus.Leader {
			return n.Status.Addr
		}
	}
	return ""
}

// GetPortMap gets a nat.PortMap from an IP
// address and a slice of nat.Ports.
func GetPortMap(ip string, ports []nat.Port) nat.PortMap {
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

// GetContainerFilterArgs gets a filters.GetContainerFilterArgs
// object that filters for a specific container by name.
func GetContainerFilterArgs(name string) filters.Args {
	args := filters.NewArgs()
	args.Add(
		"type",
		"container",
	)
	args.Add(
		"container",
		name,
	)
	return args
}

// -----------------------------------------------
// -FUNCTIONS BELOW ARE USED FOR TESTING PURPOSES-
// -----------------------------------------------

// GetAuxID gets the ID of the aux-services
// container.
func GetAuxID() string {
	ctx, cancel := context.WithCancel(context.Background())
	dockerClient, err := client.NewEnvClient()
	defer cancel()
	if err != nil {
		return ""
	}

	start := time.Now()
	for time.Since(start) < 10*time.Second {
		containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{})
		if err == nil {
			for _, c := range containers {
				split := strings.Split(c.Names[0], "/")
				if split[len(split)-1] == "aux-services" {
					return c.ID
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return ""
}

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

// AuxServiceSpec is a spec used for testing the aux service
var AuxServiceSpec = swarm.ServiceSpec{
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
				GetEnvByKey("TAG"),
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

// StartAux starts the test auxiliary services container
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

// KillAux kills the aux services container
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

// KillNet removes the test network
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

// Net ...
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
	if err != nil {
		panic(err)
	}
	if len(nets) == 1 {
		return nets[0]
	}
	return types.NetworkResource{}
}()

// AuxContainerConfig ...
var AuxContainerConfig = container.Config{
	Hostname: AuxContainerName,
	Env: []string{
		GetEnvByKey("STAGE"),
		GetEnvByKey("LOGLEVEL"),
	},
	ExposedPorts: GetPortSet(DefaultPorts),
	Image: ConcatString(
		"ramrodpcp/auxiliary-services",
		":",
		GetTagFromEnv(),
	),
}

// AuxHostConfig ...
var AuxHostConfig = container.HostConfig{
	NetworkMode: "default",
	PortBindings: GetPortMap(
		GetIP(),
		GetPorts([][]string{
			[]string{"tcp", "20"},
			[]string{"tcp", "21"},
			[]string{"tcp", "80"},
			[]string{"udp", "53"},
            []string{"tcp", "10090"},
            []string{"tcp", "10091"},
            []string{"tcp", "10092"},
            []string{"tcp", "10093"},
            []string{"tcp", "10094"},
            []string{"tcp", "10095"},
            []string{"tcp", "10096"},
            []string{"tcp", "10097"},
            []string{"tcp", "10098"},
            []string{"tcp", "10099"},
        	[]string{"tcp", "10100"},
		}),
	),
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

// AuxNetConfig ...
var AuxNetConfig = network.NetworkingConfig{
	EndpointsConfig: map[string]*network.EndpointSettings{
		"pcp": &network.EndpointSettings{
			Aliases:   []string{"auxiliary-services"},
			NetworkID: Net.ID,
		},
	},
}

// AuxContainerName ...
var AuxContainerName = "aux-services"
