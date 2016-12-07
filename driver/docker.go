package driver

import (
	"fmt"

	"github.com/estesp/dockerbench/utils"
)

// DockerDriver is an implementation of the driver interface for the Docker engine
type DockerDriver struct {
	dockerBinary string
}

// DockerContainer is an implementation of the container metadata needed for docker
type DockerContainer struct {
	name      string
	imageName string
	detached  bool
}

// NewDockerDriver creates an instance of the docker driver, providing a path to the docker client binary
func NewDockerDriver(binaryPath string) (Driver, error) {
	resolvedBinPath, err := utils.ResolveBinary(binaryPath)
	if err != nil {
		return &DockerDriver{}, err
	}
	driver := &DockerDriver{
		dockerBinary: resolvedBinPath,
	}
	driver.Info()
	return driver, nil
}

// newDockerContainer creates the metadata object of a docker-specific container with
// image name, container runtime name, and any required additional information
func newDockerContainer(name, image string, detached bool) Container {
	return &DockerContainer{
		name:      name,
		imageName: image,
		detached:  detached,
	}
}

// Name returns the name of the container
func (c *DockerContainer) Name() string {
	return c.name
}

// Detached returns whether the container should be started in detached mode
func (c *DockerContainer) Detached() bool {
	return c.detached
}

// Image returns the image name that Docker will use
func (c *DockerContainer) Image() string {
	return c.imageName
}

// Type returns a driver.Type to indentify the driver implementation
func (d *DockerDriver) Type() Type {
	return Docker
}

// Info returns
func (d *DockerDriver) Info() (string, error) {
	info := "docker driver (binary: " + d.dockerBinary + ")\n"
	versionInfo, err := utils.ExecCmd(d.dockerBinary, "version")
	if err != nil {
		return "", fmt.Errorf("Error trying to retrieve runc version info: %v", err)
	}
	return info + versionInfo, nil
}

// Create will create a container instance matching the specific needs
// of a driver
func (d *DockerDriver) Create(name, image string, detached bool) (Container, error) {
	return newDockerContainer(name, image, detached), nil
}

// Clean will clean the environment; removing any remaining containers in the runc metadata
func (d *DockerDriver) Clean() error {
	return nil
}

// Run will execute a container using the driver
func (d *DockerDriver) Run(ctr Container) (string, int, error) {
	var detached string
	if ctr.Detached() {
		detached = "-d"
	}
	args := fmt.Sprintf("run %s --name %s %s", detached, ctr.Name(), ctr.Image())
	return utils.ExecTimedCmd(d.dockerBinary, args)
}

// Stop will stop/kill a container
func (d *DockerDriver) Stop(ctr Container) (string, int, error) {
	return utils.ExecTimedCmd(d.dockerBinary, "stop "+ctr.Name())
}

// Remove will remove a container
func (d *DockerDriver) Remove(ctr Container) (string, int, error) {
	return utils.ExecTimedCmd(d.dockerBinary, "rm "+ctr.Name())
}

// Pause will pause a container
func (d *DockerDriver) Pause(ctr Container) (string, int, error) {
	return utils.ExecTimedCmd(d.dockerBinary, "pause "+ctr.Name())
}

// Unpause will unpause/resume a container
func (d *DockerDriver) Unpause(ctr Container) (string, int, error) {
	return utils.ExecTimedCmd(d.dockerBinary, "unpause "+ctr.Name())
}
