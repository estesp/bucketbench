package driver

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	docker "github.com/docker/docker/client"
	"github.com/pkg/errors"
)

const (
	dockerContainerStopTimeout = 30 * time.Second
	dockerDefaultPIDPath       = "/var/run/docker.pid"
)

// DockerDriver is an implementation of the driver interface for the Docker engine using API
type DockerDriver struct {
	client    *docker.Client
	logConfig *container.LogConfig
}

// NewDockerDriver creates an instance of Docker API driver.
func NewDockerDriver(ctx context.Context, logDriver string, logOpts map[string]string) (*DockerDriver, error) {
	client, err := docker.NewClientWithOpts()
	if err != nil {
		return nil, err
	}

	// Make sure daemon is reachable
	ping, err := client.Ping(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "daemon is unreachable")
	}

	client.NegotiateAPIVersionPing(ping)

	driver := &DockerDriver{
		client: client,
	}

	if logDriver != "" {
		driver.logConfig = &container.LogConfig{
			Type:   logDriver,
			Config: logOpts,
		}
	}

	return driver, nil
}

// Type returns a driver.Type to indentify the driver implementation
func (d *DockerDriver) Type() Type {
	return Docker
}

// Info returns a short description about the docker server
func (d *DockerDriver) Info(ctx context.Context) (string, error) {
	info, err := d.client.Info(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to query Docker info")
	}

	return fmt.Sprintf("Docker API (name: '%s', driver: '%s', version: '%s')", info.Name, info.Driver, info.ServerVersion), nil
}

// Path returns the binary (or socket) path related to the runtime in use
func (d *DockerDriver) Path() string {
	return ""
}

// Create will pull and create a container instance matching the specific needs of a driver
func (d *DockerDriver) Create(ctx context.Context, name, image, cmdOverride string, detached bool, trace bool) (Container, error) {
	// Make sure the Docker image is available locally
	images, err := d.client.ImageList(ctx, types.ImageListOptions{
		Filters: filters.NewArgs(filters.Arg("reference", image)),
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to query image list")
	}

	if len(images) == 0 {
		reader, err := d.client.ImagePull(ctx, image, types.ImagePullOptions{})
		if err != nil {
			return nil, errors.Wrapf(err, "failed to pull image: '%s'", image)
		}

		defer reader.Close()

		// We don't want image content here, just make Docker pulling the image till end
		io.Copy(ioutil.Discard, reader)
	}

	return newDockerContainer(name, image, cmdOverride, detached, trace), nil
}

// Clean removes used Docker containers
func (d *DockerDriver) Clean(ctx context.Context) error {
	listOpts := types.ContainerListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("name", ContainerNamePrefix)),
	}

	containers, err := d.client.ContainerList(ctx, listOpts)
	if err != nil {
		return err
	}

	for _, instance := range containers {
		rmOpts := types.ContainerRemoveOptions{
			Force: true,
		}

		if err := d.client.ContainerRemove(ctx, instance.ID, rmOpts); err != nil {
			return errors.Wrapf(err, "failed to remove instance with id '%s'", instance.ID)
		}
	}

	return nil
}

// Run creates a new Docker container and sends a request to the daemon to start it
func (d *DockerDriver) Run(ctx context.Context, ctr Container) (string, time.Duration, error) {
	start := time.Now()

	var config container.Config
	var hostConfig container.HostConfig

	config.Image = ctr.Image()

	if ctr.Command() != "" {
		config.Cmd = strings.Fields(ctr.Command())
	}

	if d.logConfig != nil {
		hostConfig.LogConfig = *d.logConfig
	}

	if _, err := d.client.ContainerCreate(ctx, &config, &hostConfig, nil, ctr.Name()); err != nil {
		return "", 0, errors.Wrapf(err, "couldn't create container '%s'", ctr.Name())
	}

	opts := types.ContainerStartOptions{}
	if err := d.client.ContainerStart(ctx, ctr.Name(), opts); err != nil {
		return "", 0, errors.Wrapf(err, "failed to start container '%s'", ctr.Name())
	}

	return "", time.Since(start), nil
}

// Stop stops a container
func (d *DockerDriver) Stop(ctx context.Context, ctr Container) (string, time.Duration, error) {
	start := time.Now()

	timeout := dockerContainerStopTimeout
	if err := d.client.ContainerStop(ctx, ctr.Name(), &timeout); err != nil {
		return "", 0, errors.Wrapf(err, "failed to stop container '%s'", ctr.Name())
	}

	return "", time.Since(start), nil
}

// Remove kills and removes a container
func (d *DockerDriver) Remove(ctx context.Context, ctr Container) (string, time.Duration, error) {
	start := time.Now()

	opts := types.ContainerRemoveOptions{Force: true}
	if err := d.client.ContainerRemove(ctx, ctr.Name(), opts); err != nil {
		return "", 0, errors.Wrapf(err, "failed to remove container: '%s'", ctr.Name())
	}

	return "", time.Since(start), nil
}

// Pause pauses a container
func (d *DockerDriver) Pause(ctx context.Context, ctr Container) (string, time.Duration, error) {
	start := time.Now()

	if err := d.client.ContainerPause(ctx, ctr.Name()); err != nil {
		return "", 0, nil
	}

	return "", time.Since(start), nil
}

// Unpause unpauses a container
func (d *DockerDriver) Unpause(ctx context.Context, ctr Container) (string, time.Duration, error) {
	start := time.Now()

	if err := d.client.ContainerUnpause(ctx, ctr.Name()); err != nil {
		return "", 0, errors.Wrapf(err, "failed to unpause container: '%s'", ctr.Name())
	}

	return "", time.Since(start), nil
}

// Wait will block until container stop
func (d *DockerDriver) Wait(ctx context.Context, ctr Container) (string, time.Duration, error) {
	start := time.Now()

	waitC, errC := d.client.ContainerWait(ctx, ctr.Name(), container.WaitConditionNotRunning)

	select {
	case err := <-errC:
		return "", 0, errors.Wrapf(err, "failed to wait container: '%s'", ctr.Name())
	case <-waitC:
		return "", time.Since(start), nil
	}
}

// Close closes the transport used by Docker client
func (d *DockerDriver) Close() error {
	return d.client.Close()
}

// PID returns a process ID of Docker daemon
func (d *DockerDriver) PID() (int, error) {
	return getDockerPID("")
}

// ProcNames returns the list of process names contributing to mem/cpu usage during overhead benchmark
func (d *DockerDriver) ProcNames() []string {
	return dockerProcNames
}

// Metrics returns stats data from daemon for container
func (d *DockerDriver) Metrics(ctx context.Context, ctr Container) (interface{}, error) {
	stats, err := d.client.ContainerStats(ctx, ctr.Name(), false)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get stats for container: '%s'", ctr.Name())
	}

	defer stats.Body.Close()

	data, err := ioutil.ReadAll(stats.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read stats for container: '%s'", ctr.Name())
	}

	return data, nil
}

func getDockerPID(path string) (int, error) {
	if path == "" {
		path = dockerDefaultPIDPath
	}

	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, errors.Wrap(err, "could not read Docker pid file")
	}

	return strconv.Atoi(string(buf))
}
