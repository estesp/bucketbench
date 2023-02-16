package driver

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/estesp/bucketbench/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const defaultDockerBinary = "docker"

var dockerProcNames = []string{
	"dockerd",
	"docker-containerd",
	"docker-containerd-shim",
	"docker-proxy",
}

// DockerCLIDriver is an implementation of the driver interface for the Docker engine.
// IMPORTANT: This implementation does not protect instance metadata for thread safely.
// At this time there is no understood use case for multi-threaded use of this implementation.
type DockerCLIDriver struct {
	dockerBinary string
	dockerInfo   string
	logDriver    string
	logOpts      map[string]string
	streamStats  bool
}

// DockerContainer is an implementation of the container metadata needed for docker
type DockerContainer struct {
	name        string
	imageName   string
	cmdOverride string
	detached    bool
	trace       bool
}

// NewDockerCLIDriver creates an instance of the docker driver, providing a path to the docker client binary
func NewDockerCLIDriver(ctx context.Context, config *Config) (Driver, error) {
	binaryPath := config.Path
	if binaryPath == "" {
		binaryPath = defaultDockerBinary
	}

	resolvedBinPath, err := utils.ResolveBinary(binaryPath)
	if err != nil {
		return &DockerCLIDriver{}, err
	}

	driver := &DockerCLIDriver{
		dockerBinary: resolvedBinPath,
		logDriver:    config.LogDriver,
		logOpts:      config.LogOpts,
		streamStats:  config.StreamStats,
	}

	info, err := driver.Info(ctx)
	if err != nil {
		return nil, err
	}

	log.Debugf("running docker CLI driver: '%s', log driver: '%s'", info, config.LogDriver)
	return driver, nil
}

// newDockerContainer creates the metadata object of a docker-specific container with
// image name, container runtime name, and any required additional information
func newDockerContainer(name, image, cmd string, detached bool, trace bool) Container {
	return &DockerContainer{
		name:        name,
		imageName:   image,
		cmdOverride: cmd,
		detached:    detached,
		trace:       trace,
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

// Trace returns whether the container should be started with tracing enabled
func (c *DockerContainer) Trace() bool {
	return c.trace
}

// Image returns the image name that Docker will use
func (c *DockerContainer) Image() string {
	return c.imageName
}

// Command returns the optional overriding command that Docker will use
// when executing a container based on this container's image
func (c *DockerContainer) Command() string {
	return c.cmdOverride
}

// GetPodID return pod-id associated with container.
// only used by CRI-based drivers
func (c *DockerContainer) GetPodID() string {
	return ""
}

// Type returns a driver.Type to indentify the driver implementation
func (d *DockerCLIDriver) Type() Type {
	return DockerCLI
}

// Path returns the binary path of the docker binary in use
func (d *DockerCLIDriver) Path() string {
	return d.dockerBinary
}

// Close allows the driver to handle any resource free/connection closing
// as necessary. Docker CLI has no need to perform any actions on close.
func (d *DockerCLIDriver) Close() error {
	return nil
}

// PID returns a process ID of Docker daemon
func (d *DockerCLIDriver) PID() (int, error) {
	return getDockerPID("")
}

// Wait will block until container stop
func (d *DockerCLIDriver) Wait(ctx context.Context, ctr Container) (string, time.Duration, error) {
	return utils.ExecTimedCmd(ctx, d.dockerBinary, "wait "+ctr.Name())
}

// Info returns
func (d *DockerCLIDriver) Info(ctx context.Context) (string, error) {
	if d.dockerInfo != "" {
		return d.dockerInfo, nil
	}

	infoStart := "docker driver (binary: " + d.dockerBinary + ")\n"
	version, err := utils.ExecCmd(ctx, d.dockerBinary, "version")
	if err != nil {
		return "", errors.Wrap(err, "failed to retrieve docker daemon version")
	}
	info, err := utils.ExecCmd(ctx, d.dockerBinary, "info")
	if err != nil {
		return "", errors.Wrap(err, "failed to retrieve docker daemon info")
	}

	d.dockerInfo = infoStart + parseDaemonInfo(version, info)
	return d.dockerInfo, nil
}

// Create will create a container instance matching the specific needs
// of a driver
func (d *DockerCLIDriver) Create(ctx context.Context, name, image, cmdOverride string, detached bool, trace bool) (Container, error) {
	return newDockerContainer(name, image, cmdOverride, detached, trace), nil
}

// Clean will clean the environment; removing any exited containers
func (d *DockerCLIDriver) Clean(ctx context.Context) error {
	// clean up any containers from a prior run
	log.Info("Docker: Stopping any running containers created during bucketbench runs")
	cmd := fmt.Sprintf("docker stop `docker ps -qf name=%s`", ContainerNamePrefix)
	out, err := utils.ExecShellCmd(ctx, cmd)
	if err != nil {
		// first make sure the error isn't simply that there were no
		// containers to stop:
		if !strings.Contains(out, "requires at least 1 argument") {
			log.Warnf("Docker: Failed to stop running bb-ctr-* containers: %v (output: %s)", err, out)
		}
	}
	log.Info("Docker: Removing exited containers from bucketbench runs")
	cmd = fmt.Sprintf("docker rm -f `docker ps -aqf name=%s`", ContainerNamePrefix)
	out, err = utils.ExecShellCmd(ctx, cmd)
	if err != nil {
		// first make sure the error isn't simply that there were no
		// exited containers to remove:
		if !strings.Contains(out, "requires at least 1 argument") {
			log.Warnf("Docker: Failed to remove exited bb-ctr-* containers: %v (output: %s)", err, out)
		}
	}
	return nil
}

// Run will execute a container using the driver
func (d *DockerCLIDriver) Run(ctx context.Context, ctr Container) (string, time.Duration, error) {
	args := []string{"run"}

	if ctr.Detached() {
		args = append(args, "-d")
	}

	if d.logDriver != "" {
		args = append(args, "--log-driver", d.logDriver)

		for name, value := range d.logOpts {
			args = append(args, "--log-opt", fmt.Sprintf("%s=%s", name, value))
		}
	}

	args = append(args, "--name", ctr.Name(), ctr.Image())

	if ctr.Command() != "" {
		args = append(args, ctr.Command())
	}

	return utils.ExecTimedCmd(ctx, d.dockerBinary, strings.Join(args, " "))
}

// Stop will stop a container
func (d *DockerCLIDriver) Stop(ctx context.Context, ctr Container) (string, time.Duration, error) {
	return utils.ExecTimedCmd(ctx, d.dockerBinary, "stop "+ctr.Name())
}

// Remove will remove a container
func (d *DockerCLIDriver) Remove(ctx context.Context, ctr Container) (string, time.Duration, error) {
	return utils.ExecTimedCmd(ctx, d.dockerBinary, "rm "+ctr.Name())
}

// Pause will pause a container
func (d *DockerCLIDriver) Pause(ctx context.Context, ctr Container) (string, time.Duration, error) {
	return utils.ExecTimedCmd(ctx, d.dockerBinary, "pause "+ctr.Name())
}

// Unpause will unpause/resume a container
func (d *DockerCLIDriver) Unpause(ctx context.Context, ctr Container) (string, time.Duration, error) {
	return utils.ExecTimedCmd(ctx, d.dockerBinary, "unpause "+ctr.Name())
}

// Stats returns stats data from daemon for container
func (d *DockerCLIDriver) Stats(ctx context.Context, ctr Container) (io.ReadCloser, error) {
	var args string
	if d.streamStats {
		args = "stats " + ctr.Name()
	} else {
		args = "stats --no-stream " + ctr.Name()
	}

	return utils.ExecCmdStream(ctx, d.dockerBinary, args)
}

// ProcNames returns the list of process names contributing to mem/cpu usage during overhead benchmark
func (d *DockerCLIDriver) ProcNames() []string {
	return dockerProcNames
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
