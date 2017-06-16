package driver

import (
	"context"
	"fmt"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
)

const defaultContainerdPath = "/run/containerd/containerd.sock"

// ContainerdDriver is an implementation of the driver interface for using Containerd.
// This uses the provided client library which abstracts using the gRPC APIs directly.
// IMPORTANT: This implementation does not protect instance metadata for thread safely.
// At this time there is no understood use case for multi-threaded use of this implementation.
type ContainerdDriver struct {
	ctrdAddress string
	client      *containerd.Client
	context     context.Context
}

// ContainerdContainer is an implementation of the container metadata needed for containerd
type ContainerdContainer struct {
	name      string
	imageName string
	state     string
	process   string
	trace     bool
}

// NewContainerdDriver creates an instance of the containerd driver, providing a path to the ctr client
func NewContainerdDriver(path string) (Driver, error) {
	if path == "" {
		path = defaultContainerdPath
	}
	client, err := containerd.New(path)
	if err != nil {
		return &ContainerdDriver{}, err
	}
	bbCtx := namespaces.WithNamespace(context.Background(), "bb")
	driver := &ContainerdDriver{
		ctrdAddress: path,
		client:      client,
		context:     bbCtx,
	}
	return driver, nil
}

// newContainerdContainer creates the metadata object of a containerd-specific container with
// bundle, name, and any required additional information
func newContainerdContainer(name, image string, trace bool) Container {
	return &ContainerdContainer{
		name:      name,
		imageName: image,
		trace:     trace,
	}
}

// Name returns the name of the container
func (c *ContainerdContainer) Name() string {
	return c.name
}

// Trace returns whether the container should be started with tracing enabled
func (c *ContainerdContainer) Trace() bool {
	return c.trace
}

// Image returns the bundle path that runc will use
func (c *ContainerdContainer) Image() string {
	return c.imageName
}

// Process returns the process name in cases where this container instance is
// wrapping a potentially running container
func (c *ContainerdContainer) Process() string {
	return c.process
}

// State returns the queried state of the container (if available)
func (c *ContainerdContainer) State() string {
	return c.state
}

// Detached always returns true for containerd as IO streams are always detached
func (c *ContainerdContainer) Detached() bool {
	return true
}

// Type returns a driver.Type to indentify the driver implementation
func (r *ContainerdDriver) Type() Type {
	return Containerd
}

// Info returns
func (r *ContainerdDriver) Info() (string, error) {
	version, err := r.client.Version(r.context)
	if err != nil {
		return "", err
	}
	info := "containerd gRPC client driver (daemon: " + version.Version + "[Revision: " + version.Revision + "] )"
	return info, nil
}

// Create will create a container instance matching the specific needs
// of a driver
func (r *ContainerdDriver) Create(name, image string, detached bool, trace bool) (Container, error) {
	if _, err := r.client.GetImage(r.context, image); err != nil {
		// if the image isn't already in our namespaced context, then pull it
		// using the reference and default resolver (most likely DockerHub)
		if _, err := r.client.Pull(r.context, image, containerd.WithPullUnpack); err != nil {
			// error pulling the image
			return nil, err
		}
	}
	return newContainerdContainer(name, image, trace), nil
}

// Clean will clean the environment; removing any remaining containers in the runc metadata
func (r *ContainerdDriver) Clean() error {
	var tries int
	list, err := r.client.Containers(r.context)
	if err != nil {
		return fmt.Errorf("Error getting containerd list output: %v", err)
	}
	// try up to 3 times to handle any active containers in the list
	log.Infof("Attempting to cleanup containerd containers/metadata; %d listed", len(list))
	for len(list) > 0 && tries < 3 {
		log.Infof("containerd cleanup: Pass #%d", tries+1)
		// kill/stop and remove containers
		for _, ctr := range list {
			if err := ctr.Delete(r.context); err != nil {
				log.Errorf("Error deleting container %v: %v", ctr, err)
			}
		}
		tries++
		list, err = r.client.Containers(r.context)
		if err != nil {
			return fmt.Errorf("Error getting containerd list output: %v", err)
		}
	}
	log.Infof("containerd cleanup complete.")
	return nil
}

// Run will execute a container using the containerd driver.
func (r *ContainerdDriver) Run(ctr Container) (string, int, error) {
	image, err := r.client.GetImage(r.context, ctr.Image())
	if err != nil {
		return "", 0, err
	}
	spec, err := containerd.GenerateSpec(containerd.WithImageConfig(r.context, image))
	if err != nil {
		return "", 0, err
	}
	container, err := r.client.NewContainer(r.context, ctr.Name(),
		containerd.WithSpec(spec),
		containerd.WithImage(image),
		containerd.WithNewRootFS(ctr.Name(), image))
	if err != nil {
		return "", 0, err
	}
	task, err := container.NewTask(r.context, containerd.Stdio)
	if err != nil {
		return "", 0, err
	}
	if err := task.Start(r.context); err != nil {
		task.Delete(r.context)
		return "", 0, err
	}
	return "", 0, nil
}

// Stop will stop/kill a container
func (r *ContainerdDriver) Stop(ctr Container) (string, int, error) {
	container, err := r.client.LoadContainer(r.context, ctr.Name())
	if err != nil {
		return "", 0, err
	}
	task, err := container.Task(r.context, nil)
	if err != nil {
		return "", 0, err
	}
	err = task.Kill(r.context, syscall.SIGKILL)
	if err != nil {
		return "", 0, err
	}
	_, err = task.Delete(r.context)
	if err != nil {
		return "", 0, err
	}
	return "", 0, nil
}

// Remove will remove a container; in the containerd case we simply call kill
// which will remove any container metadata if it was running
func (r *ContainerdDriver) Remove(ctr Container) (string, int, error) {
	container, err := r.client.LoadContainer(r.context, ctr.Name())
	if err != nil {
		return "", 0, err
	}
	err = container.Delete(r.context)
	if err != nil {
		return "", 0, err
	}
	return "", 0, nil
}

// Pause will pause a container
func (r *ContainerdDriver) Pause(ctr Container) (string, int, error) {
	container, err := r.client.LoadContainer(r.context, ctr.Name())
	if err != nil {
		return "", 0, err
	}
	task, err := container.Task(r.context, nil)
	if err != nil {
		return "", 0, err
	}
	err = task.Pause(r.context)
	if err != nil {
		return "", 0, err
	}
	return "", 0, nil
}

// Unpause will unpause/resume a container
func (r *ContainerdDriver) Unpause(ctr Container) (string, int, error) {
	container, err := r.client.LoadContainer(r.context, ctr.Name())
	if err != nil {
		return "", 0, err
	}
	task, err := container.Task(r.context, nil)
	if err != nil {
		return "", 0, err
	}
	err = task.Resume(r.context)
	if err != nil {
		return "", 0, err
	}
	return "", 0, nil
}
