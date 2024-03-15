package driver

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/estesp/bucketbench/utils"
	log "github.com/sirupsen/logrus"
)

const defaultYoukiBinary = "youki"

// YoukiDriver is an implementation of the driver interface for Youki.
// IMPORTANT: This implementation does not protect instance metadata for thread safely.
// At this time there is no understood use case for multi-threaded use of this implementation.
type YoukiDriver struct {
	youkiBinary string
}

// YoukiContainer is an implementation of the container metadata needed for youki
type YoukiContainer struct {
	name       string
	bundlePath string
	detached   bool
	state      string
	pid        string
	trace      bool
}

// NewYoukiDriver creates an instance of the youki driver, providing a path to youki
func NewYoukiDriver(binaryPath string) (Driver, error) {
	if binaryPath == "" {
		binaryPath = defaultYoukiBinary
	}
	resolvedBinPath, err := utils.ResolveBinary(binaryPath)
	if err != nil {
		return &YoukiDriver{}, err
	}
	driver := &YoukiDriver{
		youkiBinary: resolvedBinPath,
	}
	return driver, nil
}

// newYoukiContainer creates the metadata object of a youki-specific container with
// bundle, name, and any required additional information
func newYoukiContainer(name, bundlepath string, detached bool, trace bool) Container {
	return &YoukiContainer{
		name:       name,
		bundlePath: bundlepath,
		detached:   detached,
		trace:      trace,
	}
}

// Name returns the name of the container
func (c *YoukiContainer) Name() string {
	return c.name
}

// Detached returns whether the container should be started in detached mode
func (c *YoukiContainer) Detached() bool {
	return c.detached
}

// Trace returns whether the container should be started with tracing enabled
func (c *YoukiContainer) Trace() bool {
	return c.trace
}

// Image returns the bundle path that youki will use
func (c *YoukiContainer) Image() string {
	return c.bundlePath
}

// Command is not implemented for the youki driver type
// as the command is embedded in the config.json of the rootfs
func (c *YoukiContainer) Command() string {
	return ""
}

// Pid returns the process ID in cases where this container instance is
// wrapping a potentially running container
func (c *YoukiContainer) Pid() string {
	return c.pid
}

// State returns the queried state of the container (if available)
func (c *YoukiContainer) State() string {
	return c.state
}

// GetPodID return pod-id associated with container.
// only used by CRI-based drivers
func (c *YoukiContainer) GetPodID() string {
	return ""
}

// Type returns a driver.Type to indentify the driver implementation
func (r *YoukiDriver) Type() Type {
	return Youki
}

// Path returns the binary path of the youki binary in use
func (r *YoukiDriver) Path() string {
	return r.youkiBinary
}

// Close allows the driver to handle any resource free/connection closing
// as necessary. youki has no need to perform any actions on close.
func (r *YoukiDriver) Close() error {
	return nil
}

// PID returns daemon process id
func (r *YoukiDriver) PID() (int, error) {
	return 0, errors.New("not implemented")
}

// Wait will block until container stop
func (r *YoukiDriver) Wait(_ context.Context, _ Container) (string, time.Duration, error) {
	return "", 0, errors.New("not implemented")
}

// Stats returns stats data from daemon for container
func (r *YoukiDriver) Stats(_ context.Context, _ Container) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

// ProcNames returns the list of process names contributing to mem/cpu usage during overhead benchmark
func (r *YoukiDriver) ProcNames() []string {
	return []string{}
}

// Info returns
func (r *YoukiDriver) Info(ctx context.Context) (string, error) {
	info := "youki driver (binary: " + r.youkiBinary + ")\n"
	versionInfo, err := utils.ExecCmd(ctx, r.youkiBinary, "--version")
	if err != nil {
		return "", fmt.Errorf("Error trying to retrieve youki version info: %v", err)
	}
	return info + versionInfo, nil
}

// Create will create a container instance matching the specific needs
// of a driver
func (r *YoukiDriver) Create(_ context.Context, name, image, _ string, _ bool, _ bool) (Container, error) {
	return newYoukiContainer(name, image, false, false), nil
}

// Clean will clean the environment; removing any remaining containers in the youki metadata
func (r *YoukiDriver) Clean(ctx context.Context) error {
	var tries int
	out, err := utils.ExecCmd(ctx, r.youkiBinary, "list")
	if err != nil {
		return fmt.Errorf("Error getting youki list output: (err: %v) output: %s", err, out)
	}
	// try up to 3 times to handle any remaining containers in the youki list
	containers := parseYoukiList(out)
	log.Infof("Attempting to cleanup youki containers/metadata; %d listed", len(containers))
	for len(containers) > 0 && tries < 3 {
		log.Infof("youki cleanup: Pass #%d", tries+1)
		for _, ctr := range containers {
			switch ctr.State() {
			case "Running":
				log.Infof("Attempting stop and remove on container %q", ctr.Name())
				r.Stop(ctx, ctr)
				r.Remove(ctx, ctr)
			case "Paused":
				log.Infof("Attempting unpause and removal of container %q", ctr.Name())
				r.Unpause(ctx, ctr)
				r.Remove(ctx, ctr)
			case "Stopped":
				log.Infof("Attempting remove of container %q", ctr.Name())
				r.Remove(ctx, ctr)
			default:
				log.Warnf("Unknown state %q for ctr %q", ctr.State(), ctr.Name())
			}
		}
		tries++
		out, err := utils.ExecCmd(ctx, r.youkiBinary, "list")
		if err != nil {
			return fmt.Errorf("Error getting youki list output: %v", err)
		}
		containers = parseYoukiList(out)
	}
	log.Infof("youki cleanup complete.")
	return nil
}

// Run will execute a container using the driver.Youki automatically uses detach mode.
func (r *YoukiDriver) Run(ctx context.Context, ctr Container) (string, time.Duration, error) {

	args := fmt.Sprintf("run --bundle %s %s", ctr.Image(), ctr.Name())
	// the "NoOut" variant of ExecTimedCmd ignores stdin/out/err (sets them to /dev/null)
	return utils.ExecTimedCmdNoOut(ctx, r.youkiBinary, args)
}

// Stop will stop/kill a container
func (r *YoukiDriver) Stop(ctx context.Context, ctr Container) (string, time.Duration, error) {
	return utils.ExecTimedCmd(ctx, r.youkiBinary, "kill "+ctr.Name()+" KILL")
}

// Remove will remove a container
func (r *YoukiDriver) Remove(ctx context.Context, ctr Container) (string, time.Duration, error) {
	return utils.ExecTimedCmd(ctx, r.youkiBinary, "delete "+ctr.Name())
}

// Pause will pause a container
func (r *YoukiDriver) Pause(ctx context.Context, ctr Container) (string, time.Duration, error) {
	return utils.ExecTimedCmd(ctx, r.youkiBinary, "pause "+ctr.Name())
}

// Unpause will unpause/resume a container
func (r *YoukiDriver) Unpause(ctx context.Context, ctr Container) (string, time.Duration, error) {
	return utils.ExecTimedCmd(ctx, r.youkiBinary, "resume "+ctr.Name())
}

// take the output of "youki list" and parse into container instances
func parseYoukiList(listOutput string) []*YoukiContainer {
	var results []*YoukiContainer
	reader := strings.NewReader(listOutput)
	scan := bufio.NewScanner(reader)

	for scan.Scan() {
		line := scan.Text()
		if strings.HasPrefix(line, "ID ") {
			// skip header line
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 4 {
			// not sure what this is, but it ain't a container
			log.Warnf("youki list parsing found invalid line: %q", line)
			continue
		}
		// don't delete containers that aren't part of our benchmark run!
		if !strings.Contains(parts[0], "bb-") {
			continue
		}
		ctr := &YoukiContainer{
			name:       parts[0],
			bundlePath: parts[3],
			pid:        parts[1],
			state:      parts[2],
		}
		results = append(results, ctr)
	}
	return results
}
