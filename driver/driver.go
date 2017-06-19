package driver

import "fmt"

// Type represents the know implementations of the driver interface
type Type int

const (
	// Docker represents the Docker driver implementation
	Docker Type = iota
	// Runc represents the runc-based driver implementation
	Runc
	// Containerd represents the containerd-based driver implementation
	// using the GRPC API via the containerd client library
	Containerd
	// Ctr represents the containerd legacy driver using the `ctr`
	// binary to drive containerd operations
	Ctr
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
}

// Driver is an interface for various container engines. The integer returned from
// container operations is the milliseconds elapsed for any command
type Driver interface {

	// Type returns a driver type to identify the driver
	Type() Type

	// Info returns a string with information about the container engine/runtime details
	Info() (string, error)

	// Create will create a container instance matching the specific needs
	// of a driver
	Create(name, image, cmdOverride string, detached bool, trace bool) (Container, error)

	// Clean will clean the operating environment of a specific driver
	Clean() error

	// Run will execute a container using the driver
	Run(ctr Container) (string, int, error)

	// Stop will stop/kill a container
	Stop(ctr Container) (string, int, error)

	// Remove will remove a container
	Remove(ctr Container) (string, int, error)

	// Pause will pause a container
	Pause(ctr Container) (string, int, error)

	// Unpause will unpause/resume a container
	Unpause(ctr Container) (string, int, error)
}

// New creates a driver instance of a specific type
func New(dtype Type, path string) (Driver, error) {
	switch dtype {
	case Runc:
		return NewRuncDriver(path)
	case Docker:
		return NewDockerDriver(path)
	case Containerd:
		return NewContainerdDriver(path)
	case Ctr:
		return NewCtrDriver(path)
	case Null:
		return nil, nil
	default:
		return nil, fmt.Errorf("No such driver type: %v", dtype)
	}
}

// TypeToString converts a driver Type into its string representation
func TypeToString(dtype Type) string {
	var driverType string
	switch dtype {
	case Docker:
		driverType = "Docker"
	case Containerd:
		driverType = "Containerd"
	case Ctr:
		driverType = "Ctr"
	case Runc:
		driverType = "Runc"
	default:
		driverType = "(unknown)"
	}
	return driverType
}

// StringToType converts a driver stringified typename into its Type
func StringToType(dtype string) Type {
	var driverType Type
	switch dtype {
	case "Docker":
		driverType = Docker
	case "Containerd":
		driverType = Containerd
	case "Ctr":
		driverType = Ctr
	case "Runc":
		driverType = Runc
	default:
		driverType = Null
	}
	return driverType
}
