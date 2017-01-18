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
	Containerd
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
	Create(name, image string, detached bool, trace bool) (Container, error)

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
func New(dtype Type, binaryPath string) (Driver, error) {
	switch dtype {
	case Runc:
		return NewRuncDriver(binaryPath)
	case Docker:
		return NewDockerDriver(binaryPath)
	case Containerd:
		return nil, fmt.Errorf("Containerd driver unimplemented")
	case Null:
		return nil, nil
	default:
		return nil, fmt.Errorf("No such driver type: %v", dtype)
	}
}
