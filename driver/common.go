package driver

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Type represents the know implementations of the driver interface
type Type int

// ContainerNamePrefix represents containers name prefix
const ContainerNamePrefix = "bb-ctr"

const (
	// DockerCLI represents the Docker CLI driver implementation
	DockerCLI Type = iota
	// Docker represents the Docker API driver implementation
	Docker
	// Runc represents the runc-based driver implementation
	Runc
	// Containerd represents the containerd-based driver implementation
	// using the GRPC API via the containerd client library
	Containerd
	// Ctr represents the containerd legacy driver using the `ctr`
	// binary to drive containerd operations
	Ctr
	// CRI driver represents k8s Container Runtime Interface
	CRI
	// Null driver represents an empty driver for use by benchmarks that
	// require no driver
	Null
)

// Container represents a generic container instance on any container engine
type Container interface {
	// Name returns the name of the container
	Name() string

	// Detached returns whether the container is to be started in detached state
	Detached() bool

	// Trace returns whether the container should be traced (using any tracing supported
	// by the container runtime)
	Trace() bool

	// Image returns either a bundle path (used by runc, containerd) or image name (used by Docker)
	// that will be used by the container runtime to know what image to run/execute
	Image() string

	// Command returns an optional command that overrides the default image
	// "CMD" or "ENTRYPOINT" for the Docker and Containerd (gRPC) drivers
	Command() string

	//GetPodID returns podid associated with the container
	//only used by CRI-based drivers
	GetPodID() string
}

// Driver is an interface for various container engines. The integer returned from
// container operations is the milliseconds elapsed for any command
type Driver interface {

	// Type returns a driver type to identify the driver
	Type() Type

	// Info returns a string with information about the container engine/runtime details
	Info(ctx context.Context) (string, error)

	// Path returns the binary (or socket) path related to the runtime in use
	Path() string

	// Create will create a container instance matching the specific needs
	// of a driver
	Create(ctx context.Context, name, image, cmdOverride string, detached bool, trace bool) (Container, error)

	// Clean will clean the operating environment of a specific driver
	Clean(ctx context.Context) error

	// Run will execute a container using the driver
	Run(ctx context.Context, ctr Container) (string, time.Duration, error)

	// Stop will stop/kill a container
	Stop(ctx context.Context, ctr Container) (string, time.Duration, error)

	// Remove will remove a container
	Remove(ctx context.Context, ctr Container) (string, time.Duration, error)

	// Pause will pause a container
	Pause(ctx context.Context, ctr Container) (string, time.Duration, error)

	// Unpause will unpause/resume a container
	Unpause(ctx context.Context, ctr Container) (string, time.Duration, error)

	// Wait blocks thread until container stop
	Wait(ctx context.Context, ctr Container) (string, time.Duration, error)

	// Close allows the driver to free any resources/close any
	// connections
	Close() error

	// PID returns daemon process id
	PID() (int, error)

	// ProcNames returns the list of process names contributing to mem/cpu usage during overhead benchmark
	ProcNames() []string

	// Stats returns a reader with streaming data output
	Stats(ctx context.Context, ctr Container) (io.ReadCloser, error)
}

// Config represents various configuration flags for driver
type Config struct {
	DriverType    Type
	Path          string
	LogDriver     string
	LogOpts       map[string]string
	StreamStats   bool
	StatsInterval time.Duration
}

// New creates a driver instance of a specific type
func New(ctx context.Context, config *Config) (Driver, error) {
	switch config.DriverType {
	case Runc:
		return NewRuncDriver(config.Path)
	case DockerCLI:
		return NewDockerCLIDriver(ctx, config)
	case Docker:
		return NewDockerDriver(ctx, config)
	case Containerd:
		return NewContainerdDriver(config)
	case Ctr:
		return NewCtrDriver(config.Path)
	case CRI:
		return NewCRIDriver(config.Path)
	case Null:
		return nil, nil
	default:
		return nil, fmt.Errorf("no such driver type: %s", config.DriverType)
	}
}

// String converts a driver Type into its string representation
func (driverType Type) String() string {
	switch driverType {
	case DockerCLI:
		return "DockerCLI"
	case Docker:
		return "Docker"
	case Containerd:
		return "Containerd"
	case Ctr:
		return "Ctr"
	case Runc:
		return "Runc"
	case CRI:
		return "CRI"
	default:
		return "(unknown)"
	}
}

// StringToType converts a driver stringified typename into its Type
func StringToType(dtype string) Type {
	var driverType Type
	switch dtype {
	case "DockerCLI":
		driverType = DockerCLI
	case "Docker":
		driverType = Docker
	case "Containerd":
		driverType = Containerd
	case "Ctr":
		driverType = Ctr
	case "Runc":
		driverType = Runc
	case "CRI":
		driverType = CRI
	default:
		driverType = Null
	}
	return driverType
}
