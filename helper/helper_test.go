package helper

import (
	"net"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
)

func Test_ConcatString(t *testing.T) {
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
			if got := ConcatString(tt.args.k, tt.args.sep, tt.args.v); got != tt.want {
				t.Errorf("concatString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GetEnvByKey(t *testing.T) {
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
			res := GetEnvByKey(tt.k)
			assert.Equal(t, tt.want, res)
			os.Setenv(tt.k, "")
		})
	}
}

func Test_GetTagFromEnv(t *testing.T) {
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
			assert.Equal(t, tt.want, GetTagFromEnv())
			os.Setenv("TAG", old)
		})
	}
}

func Test_GetPorts(t *testing.T) {
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
			if got := GetPorts(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPorts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GetPortSet(t *testing.T) {
	tests := []struct {
		name string
		want nat.PortSet
	}{
		{
			name: "Normal 1",
			want: nat.PortSet{
				"20/tcp":    struct{}{},
				"21/tcp":    struct{}{},
				"80/tcp":    struct{}{},
				"53/udp":    struct{}{},
				"10090/tcp": struct{}{},
				"10091/tcp": struct{}{},
				"10092/tcp": struct{}{},
				"10093/tcp": struct{}{},
				"10094/tcp": struct{}{},
				"10095/tcp": struct{}{},
				"10096/tcp": struct{}{},
				"10097/tcp": struct{}{},
				"10098/tcp": struct{}{},
				"10099/tcp": struct{}{},
				"10100/tcp": struct{}{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetPortSet(DefaultPorts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPortSet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GetIP(t *testing.T) {

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
			got := GetIP()
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

func Test_GetPortMap(t *testing.T) {
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
			if got := GetPortMap(tt.args.ip, tt.args.ports); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPortMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GetContainerFilterArgs(t *testing.T) {

	f := filters.NewArgs()
	f.Add(
		"type", "container",
	)
	f.Add(
		"container", AuxContainerName,
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
			if got := GetContainerFilterArgs(AuxContainerName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}
