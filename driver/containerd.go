package driver

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	log "github.com/sirupsen/logrus"
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
	name        string
	imageName   string
	cmdOverride string
	state       string
	process     string
	trace       bool
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
func newContainerdContainer(name, image, cmd string, trace bool) Container {
	return &ContainerdContainer{
		name:        name,
		imageName:   image,
		cmdOverride: cmd,
		trace:       trace,
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

// Command returns the override command that will be executed instead of
// the default image-specified command
func (c *ContainerdContainer) Command() string {
	return c.cmdOverride
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

// Path returns the address (socket path) of the gRPC containerd API endpoint
func (r *ContainerdDriver) Path() string {
	return r.ctrdAddress
}

// Close allows the driver to handle any resource free/connection closing
// as necessary.
func (r *ContainerdDriver) Close() error {
	if err := r.client.Close(); err != nil {
		return err
	}
	return nil
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
func (r *ContainerdDriver) Create(name, image, cmdOverride string, detached bool, trace bool) (Container, error) {
	// we need to convert the bare Docker image name to a fully resolved
	// reference (since the Docker driver and containerd driver share image
	// name references)
	fullImageName := resolveDockerImageName(image)
	if _, err := r.client.GetImage(r.context, fullImageName); err != nil {
		// if the image isn't already in our namespaced context, then pull it
		// using the reference and default resolver (most likely DockerHub)
		if _, err := r.client.Pull(r.context, fullImageName, containerd.WithPullUnpack); err != nil {
			// error pulling the image
			return nil, err
		}
	}
	return newContainerdContainer(name, fullImageName, cmdOverride, trace), nil
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
			if err := stopTask(r.context, ctr); err != nil {
				log.Errorf("Error stopping container: %v", err)
			}
			if err := ctr.Delete(r.context, containerd.WithSnapshotCleanup); err != nil {
				log.Errorf("Error deleting container %v", err)
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
	start := time.Now()
	image, err := r.client.GetImage(r.context, ctr.Image())
	if err != nil {
		return "", 0, err
	}
	var spec *specs.Spec
	if ctr.Command() != "" {
		// the command needs to be overridden in the generated spec
		spec, err = containerd.GenerateSpec(containerd.WithImageConfig(r.context, image),
			containerd.WithProcessArgs(strings.Split(ctr.Command(), " ")...))
	} else {
		spec, err = containerd.GenerateSpec(containerd.WithImageConfig(r.context, image))
	}
	if err != nil {
		return "", 0, err
	}
	container, err := r.client.NewContainer(r.context, ctr.Name(),
		containerd.WithSpec(spec),
		containerd.WithImage(image),
		containerd.WithNewSnapshot(ctr.Name(), image))
	if err != nil {
		return "", 0, err
	}

	stdouterr := bytes.NewBuffer(nil)
	task, err := container.NewTask(r.context, containerd.NewIO(bytes.NewBuffer(nil), stdouterr, stdouterr))
	if err != nil {
		return "", 0, err
	}
	if err := task.Start(r.context); err != nil {
		task.Delete(r.context)
		return "", 0, err
	}
	elapsed := time.Since(start)
	msElapsed := int(elapsed.Nanoseconds() / 1000000)
	return stdouterr.String(), msElapsed, nil
}

// Stop will stop/kill a container (specifically, the tasks [processes]
// running in the container)
func (r *ContainerdDriver) Stop(ctr Container) (string, int, error) {
	start := time.Now()
	container, err := r.client.LoadContainer(r.context, ctr.Name())
	if err != nil {
		return "", 0, err
	}
	if err = stopTask(r.context, container); err != nil {
		return "", 0, err
	}
	elapsed := time.Since(start)
	msElapsed := int(elapsed.Nanoseconds() / 1000000)
	return "", msElapsed, nil
}

// Remove will remove a container; in the containerd case we simply call kill
// which will remove any container metadata if it was running
func (r *ContainerdDriver) Remove(ctr Container) (string, int, error) {
	start := time.Now()
	container, err := r.client.LoadContainer(r.context, ctr.Name())
	if err != nil {
		return "", 0, err
	}
	err = container.Delete(r.context, containerd.WithSnapshotCleanup)
	if err != nil {
		return "", 0, err
	}
	elapsed := time.Since(start)
	msElapsed := int(elapsed.Nanoseconds() / 1000000)
	return "", msElapsed, nil
}

// Pause will pause a container
func (r *ContainerdDriver) Pause(ctr Container) (string, int, error) {
	start := time.Now()
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
	elapsed := time.Since(start)
	msElapsed := int(elapsed.Nanoseconds() / 1000000)
	return "", msElapsed, nil
}

// Unpause will unpause/resume a container
func (r *ContainerdDriver) Unpause(ctr Container) (string, int, error) {
	start := time.Now()
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
	elapsed := time.Since(start)
	msElapsed := int(elapsed.Nanoseconds() / 1000000)
	return "", msElapsed, nil
}

// much of this code is copied from docker/docker/reference.go
const (
	// DefaultTag defines the default tag used when performing images related actions and no tag or digest is specified
	DefaultTag = "latest"
	// DefaultHostname is the default built-in hostname
	DefaultHostname = "docker.io"
	// DefaultRepoPrefix is the prefix used for default repositories in default host
	DefaultRepoPrefix = "library/"
)

// resolve a Docker image name to a fully normalized reference with
// registry hostname and tag; note that most of this function is copied
// as-is from docker/docker/reference.go; the "stripHostname()" function
// specifically
func resolveDockerImageName(name string) string {
	var (
		hostname, remoteName string
	)
	i := strings.IndexRune(name, '/')
	if i == -1 || (!strings.ContainsAny(name[:i], ".:") && name[:i] != "localhost") {
		hostname, remoteName = DefaultHostname, name
	} else {
		hostname, remoteName = name[:i], name[i+1:]
	}
	if hostname == DefaultHostname && !strings.ContainsRune(remoteName, '/') {
		remoteName = DefaultRepoPrefix + remoteName
	}
	if !strings.ContainsRune(remoteName, ':') {
		// append default tag
		remoteName = remoteName + ":" + DefaultTag
	}
	return fmt.Sprintf("%s/%s", hostname, remoteName)
}

// common code for task stop/kill using the containerd gRPC API
func stopTask(ctx context.Context, ctr containerd.Container) error {
	task, err := ctr.Task(ctx, nil)
	if err != nil {
		if !strings.Contains(err.Error(), "no running task") {
			return err
		}
		//nothing to do; no task running
		return nil
	}
	status, err := task.Status(ctx)
	switch status {
	case containerd.Stopped:
		_, err := task.Delete(ctx)
		if err != nil {
			return err
		}
	case containerd.Running:
		statusC := make(chan uint32, 1)
		go func() {
			status, err := task.Wait(ctx)
			if err != nil {
				log.Errorf("container %q: error during wait: %v", ctr.ID(), err)
			}
			statusC <- status
		}()
		err := task.Kill(ctx, syscall.SIGKILL)
		if err != nil {
			task.Delete(ctx)
			return err
		}
		status := <-statusC
		if status != 0 {
			log.Debugf("%s: exited container process: code: %d", ctr.ID(), status)
		}
		_, err = task.Delete(ctx)
		if err != nil {
			return err
		}
	case containerd.Paused:
		return fmt.Errorf("Can't stop a paused container; unpause first")
	}
	return nil
}
