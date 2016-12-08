package driver

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/estesp/dockerbench/utils"
	log "github.com/sirupsen/logrus"
)

// DockerDriver is an implementation of the driver interface for the Docker engine
type DockerDriver struct {
	dockerBinary string
	dockerInfo   string
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
	if d.dockerInfo != "" {
		return d.dockerInfo, nil
	}

	infoStart := "docker driver (binary: " + d.dockerBinary + ")\n"
	version, err := utils.ExecCmd(d.dockerBinary, "version")
	info, err := utils.ExecCmd(d.dockerBinary, "info")
	if err != nil {
		return "", fmt.Errorf("Error trying to retrieve docker daemon info: %v", err)
	}
	d.dockerInfo = infoStart + parseDaemonInfo(version, info)
	return d.dockerInfo, nil
}

// Create will create a container instance matching the specific needs
// of a driver
func (d *DockerDriver) Create(name, image string, detached bool) (Container, error) {
	return newDockerContainer(name, image, detached), nil
}

// Clean will clean the environment; removing any remaining containers in the runc metadata
func (d *DockerDriver) Clean() error {
	// make sure some default images are pulled
	log.Info("Pulling busybox image")
	out, err := utils.ExecCmd(d.dockerBinary, "pull busybox")
	if err != nil {
		return fmt.Errorf("Can't pull busybox image: %v (output: %s)", err, out)
	}
	log.Info("Pulling redis image")
	out, err = utils.ExecCmd(d.dockerBinary, "pull redis")
	if err != nil {
		return fmt.Errorf("Can't pull redis image: %v (output: %s)", err, out)
	}
	// clean up any containers from a prior run
	log.Info("Clearing docker daemon exited containers")
	cmd := "docker rm -f `docker ps -aq`"
	out, err = utils.ExecCmd("bash", "-c "+cmd)
	if err != nil {
		log.Warnf("Couldn't clean up docker daemon containers: %v (output: %s)", err, out)
	}
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

// return a condensed string of version and daemon information
func parseDaemonInfo(version, info string) string {
	var (
		clientVer string
		clientAPI string
		serverVer string
	)
	vReader := strings.NewReader(version)
	vScan := bufio.NewScanner(vReader)

	for vScan.Scan() {
		line := vScan.Text()
		parts := strings.Split(line, ":")
		switch strings.TrimSpace(parts[0]) {
		case "Version":
			if clientVer == "" {
				// first time is client
				clientVer = strings.TrimSpace(parts[1])
			} else {
				serverVer = strings.TrimSpace(parts[1])
			}
		case "API version":
			if clientAPI == "" {
				// first instance is client
				clientAPI = parts[1]
				clientVer = clientVer + "|API:" + strings.TrimSpace(parts[1])
			} else {
				serverVer = serverVer + "|API:" + strings.TrimSpace(parts[1])
			}
		default:
		}

	}
	iReader := strings.NewReader(info)
	iScan := bufio.NewScanner(iReader)

	for iScan.Scan() {
		line := iScan.Text()
		parts := strings.Split(line, ":")
		switch strings.TrimSpace(parts[0]) {
		case "Kernel Version":
			serverVer = serverVer + "|Kernel:" + strings.TrimSpace(parts[1])
		case "Storage Driver":
			serverVer = serverVer + "|Storage:" + strings.TrimSpace(parts[1])
		case "Backing Filesystem":
			serverVer = serverVer + "|BackingFS:" + strings.TrimSpace(parts[1])
		default:
		}

	}
	return fmt.Sprintf("[CLIENT:%s][SERVER:%s]", clientVer, serverVer)
}
