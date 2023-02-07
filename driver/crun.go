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

const defaultCRunBinary = "crun"

// CRunDriver is an implementation of the driver interface for CRun.
// IMPORTANT: This implementation does not protect instance metadata for thread safely.
// At this time there is no understood use case for multi-threaded use of this implementation.
type CRunDriver struct {
	crunBinary string
}

// CRunContainer is an implementation of the container metadata needed for crun
type CRunContainer struct {
	name       string
	bundlePath string
	detached   bool
	state      string
	pid        string
	trace      bool
}

// NewCRunDriver creates an instance of the crun driver, providing a path to crun
func NewCRunDriver(binaryPath string) (Driver, error) {
	if binaryPath == "" {
		binaryPath = defaultCRunBinary
	}
	resolvedBinPath, err := utils.ResolveBinary(binaryPath)
	if err != nil {
		return &CRunDriver{}, err
	}
	driver := &CRunDriver{
		crunBinary: resolvedBinPath,
	}
	return driver, nil
}

// newCRunContainer creates the metadata object of a crun-specific container with
// bundle, name, and any required additional information
func newCRunContainer(name, bundlepath string, detached bool, trace bool) Container {
	return &CRunContainer{
		name:       name,
		bundlePath: bundlepath,
		detached:   detached,
		trace:      trace,
	}
}

// Name returns the name of the container
func (c *CRunContainer) Name() string {
	return c.name
}

// Detached returns whether the container should be started in detached mode
func (c *CRunContainer) Detached() bool {
	return c.detached
}

// Trace returns whether the container should be started with tracing enabled
func (c *CRunContainer) Trace() bool {
	return c.trace
}

// Image returns the bundle path that crun will use
func (c *CRunContainer) Image() string {
	return c.bundlePath
}

// Command is not implemented for the crun driver type
// as the command is embedded in the config.json of the rootfs
func (c *CRunContainer) Command() string {
	return ""
}

// Pid returns the process ID in cases where this container instance is
// wrapping a potentially running container
func (c *CRunContainer) Pid() string {
	return c.pid
}

// State returns the queried state of the container (if available)
func (c *CRunContainer) State() string {
	return c.state
}

// GetPodID return pod-id associated with container.
// only used by CRI-based drivers
func (c *CRunContainer) GetPodID() string {
	return ""
}

// Type returns a driver.Type to indentify the driver implementation
func (r *CRunDriver) Type() Type {
	return CRun
}

// Path returns the binary path of the crun binary in use
func (r *CRunDriver) Path() string {
	return r.crunBinary
}

// Close allows the driver to handle any resource free/connection closing
// as necessary. CRun has no need to perform any actions on close.
func (r *CRunDriver) Close() error {
	return nil
}

// PID returns daemon process id
func (r *CRunDriver) PID() (int, error) {
	return 0, errors.New("not implemented")
}

// Wait will block until container stop
func (r *CRunDriver) Wait(ctx context.Context, ctr Container) (string, time.Duration, error) {
	return "", 0, errors.New("not implemented")
}

// Stats returns stats data from daemon for container
func (r *CRunDriver) Stats(ctx context.Context, ctr Container) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

// ProcNames returns the list of process names contributing to mem/cpu usage during overhead benchmark
func (r *CRunDriver) ProcNames() []string {
	return []string{}
}

// Info returns
func (r *CRunDriver) Info(ctx context.Context) (string, error) {
	info := "crun driver (binary: " + r.crunBinary + ")\n"
	versionInfo, err := utils.ExecCmd(ctx, r.crunBinary, "--v")
	if err != nil {
		return "", fmt.Errorf("Error trying to retrieve crun version info: %v", err)
	}
	return info + versionInfo, nil
}

// Create will create a container instance matching the specific needs
// of a driver
func (r *CRunDriver) Create(ctx context.Context, name, image, cmdOverride string, detached bool, trace bool) (Container, error) {
	return newCRunContainer(name, image, detached, false), nil
}

// Clean will clean the environment; removing any remaining containers in the crun metadata
func (r *CRunDriver) Clean(ctx context.Context) error {
	var tries int
	out, err := utils.ExecCmd(ctx, r.crunBinary, "list")
	if err != nil {
		return fmt.Errorf("Error getting crun list output: (err: %v) output: %s", err, out)
	}
	// try up to 3 times to handle any remaining containers in the crun list
	containers := parseCRunList(out)
	log.Infof("Attempting to cleanup crun containers/metadata; %d listed", len(containers))
	for len(containers) > 0 && tries < 3 {
		log.Infof("crun cleanup: Pass #%d", tries+1)
		for _, ctr := range containers {
			switch ctr.State() {
			case "running":
				log.Infof("Attempting stop and remove on container %q", ctr.Name())
				r.Stop(ctx, ctr)
				r.Remove(ctx, ctr)
			case "paused":
				log.Infof("Attempting unpause and removal of container %q", ctr.Name())
				r.Unpause(ctx, ctr)
				r.Remove(ctx, ctr)
			case "stopped":
				log.Infof("Attempting remove of container %q", ctr.Name())
				r.Remove(ctx, ctr)
			default:
				log.Warnf("Unknown state %q for ctr %q", ctr.State(), ctr.Name())
			}
		}
		tries++
		out, err := utils.ExecCmd(ctx, r.crunBinary, "list")
		if err != nil {
			return fmt.Errorf("Error getting crun list output: %v", err)
		}
		containers = parseCRunList(out)
	}
	log.Infof("crun cleanup complete.")
	return nil
}

// Run will execute a container using the driver. Note that if the container is specified to
// run detached, but the config.json for the bundle specifies a "tty" allocation, this
// crun invocation will fail due to the fact we cannot detach without providing a "--console"
// device to crun. Detached daemon/server bundles should not need a tty; stdin/out/err of
// the container will be ignored given this is for benchmarking not validating container
// operation.
func (r *CRunDriver) Run(ctx context.Context, ctr Container) (string, time.Duration, error) {
	var detached string
	if ctr.Detached() {
		detached = "--detach"
	}

	args := fmt.Sprintf("run %s --bundle %s %s", detached, ctr.Image(), ctr.Name())
	// the "NoOut" variant of ExecTimedCmd ignores stdin/out/err (sets them to /dev/null)
	return utils.ExecTimedCmdNoOut(ctx, r.crunBinary, args)
}

// Stop will stop/kill a container
func (r *CRunDriver) Stop(ctx context.Context, ctr Container) (string, time.Duration, error) {
	return utils.ExecTimedCmd(ctx, r.crunBinary, "kill "+ctr.Name()+" KILL")
}

// Remove will remove a container
func (r *CRunDriver) Remove(ctx context.Context, ctr Container) (string, time.Duration, error) {
	return utils.ExecTimedCmd(ctx, r.crunBinary, "delete "+ctr.Name())
}

// Pause will pause a container
func (r *CRunDriver) Pause(ctx context.Context, ctr Container) (string, time.Duration, error) {
	return utils.ExecTimedCmd(ctx, r.crunBinary, "pause "+ctr.Name())
}

// Unpause will unpause/resume a container
func (r *CRunDriver) Unpause(ctx context.Context, ctr Container) (string, time.Duration, error) {
	return utils.ExecTimedCmd(ctx, r.crunBinary, "resume "+ctr.Name())
}

// take the output of "crun list" and parse into container instances
func parseCRunList(listOutput string) []*CRunContainer {
	var results []*CRunContainer
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
			log.Warnf("crun list parsing found invalid line: %q", line)
			continue
		}
		// don't delete containers that aren't part of our benchmark run!
		if !strings.Contains(parts[0], "bb-") {
			continue
		}
		ctr := &CRunContainer{
			name:       parts[0],
			bundlePath: parts[3],
			pid:        parts[1],
			state:      parts[2],
		}
		results = append(results, ctr)
	}
	return results
}
