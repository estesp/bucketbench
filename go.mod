module github.com/estesp/bucketbench

go 1.15

require (
	github.com/Microsoft/go-winio v0.4.15 // indirect
	github.com/Microsoft/hcsshim v0.8.10 // indirect
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/containerd/cgroups v0.0.0-20200824123100-0b889c03f102
	github.com/containerd/containerd v1.4.3
	github.com/containerd/continuity v0.0.0-20201204184040-1d9893e5674b // indirect
	github.com/containerd/fifo v0.0.0-20201026212402-0724c46b320c // indirect
	github.com/containerd/ttrpc v1.0.2 // indirect
	github.com/containerd/typeurl v1.0.1 // indirect
	github.com/coreos/go-systemd/v22 v22.1.0 // indirect
	github.com/docker/distribution v2.7.1-0.20190205005809-0d3efadf0154+incompatible // indirect
	github.com/docker/docker v1.4.2-0.20200203170920-46ec8731fbce
	github.com/docker/go-connections v0.4.1-0.20190612165340-fd1b1942c4d5 // indirect
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/go-yaml/yaml v2.1.0+incompatible
	github.com/godbus/dbus v0.0.0-20151105175453-c7fdd8b5cd55 // indirect
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/golang/protobuf v1.4.3 // indirect
	github.com/google/uuid v1.1.2 // indirect
	github.com/montanaflynn/stats v0.0.0-20170404204349-41c34e4914ec
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v1.0.0-rc6.0.20181203215513-96ec2177ae84 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20200728170252-4d89ac9fbff6 // indirect
	github.com/opencontainers/selinux v1.6.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/shirou/gopsutil v0.0.0-20180916084002-77e5abb6f06f
	github.com/shirou/w32 v0.0.0-20160930032740-bb4de0191aa4 // indirect
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v0.0.2-0.20171109065643-2da4a54c5cee
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2 // indirect
	golang.org/x/net v0.0.0-20200707034311-ab3426394381 // indirect
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208 // indirect
	golang.org/x/sys v0.0.0-20201202213521-69691e467435 // indirect
	google.golang.org/grpc v1.30.0
	k8s.io/cri-api v0.19.4
)

replace (
	github.com/gogo/googleapis => github.com/gogo/googleapis v1.3.2
	github.com/golang/protobuf => github.com/golang/protobuf v1.3.5
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20200224152610-e50cd9704f63
	google.golang.org/grpc => google.golang.org/grpc v1.27.1
)
