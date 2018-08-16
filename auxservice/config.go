package auxservice

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
)

var defaultEnvs = map[string]string{
	"LOGLEVEL": "DEBUG",
	"STAGE":    "DEV",
}

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
