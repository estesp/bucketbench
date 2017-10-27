package driver

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/estesp/bucketbench/utils"
	log "github.com/sirupsen/logrus"
)

const defaultRuncBinary = "runc"

// RuncDriver is an implementation of the driver interface for Runc.
// IMPORTANT: This implementation does not protect instance metadata for thread safely.
// At this time there is no understood use case for multi-threaded use of this implementation.
type RuncDriver struct {
	runcBinary string
}

// RuncContainer is an implementation of the container metadata needed for runc
type RuncContainer struct {
	name       string
	bundlePath string
	detached   bool
	state      string
	pid        string
	trace      bool
}

// NewRuncDriver creates an instance of the runc driver, providing a path to runc
func NewRuncDriver(binaryPath string) (Driver, error) {
	if binaryPath == "" {
		binaryPath = defaultRuncBinary
	}
	resolvedBinPath, err := utils.ResolveBinary(binaryPath)
	if err != nil {
		return &RuncDriver{}, err
	}
	driver := &RuncDriver{
		runcBinary: resolvedBinPath,
	}
	return driver, nil
}

// newRuncContainer creates the metadata object of a runc-specific container with
// bundle, name, and any required additional information
func newRuncContainer(name, bundlepath string, detached bool, trace bool) Container {
	return &RuncContainer{
		name:       name,
		bundlePath: bundlepath,
		detached:   detached,
		trace:      trace,
	}
}

// Name returns the name of the container
func (c *RuncContainer) Name() string {
	return c.name
}

// Detached returns whether the container should be started in detached mode
func (c *RuncContainer) Detached() bool {
	return c.detached
}

// Trace returns whether the container should be started with tracing enabled
func (c *RuncContainer) Trace() bool {
	return c.trace
}

// Image returns the bundle path that runc will use
func (c *RuncContainer) Image() string {
	return c.bundlePath
}

// Command is not implemented for the runc driver type
// as the command is embedded in the config.json of the rootfs
func (c *RuncContainer) Command() string {
	return ""
}

// Pid returns the process ID in cases where this container instance is
// wrapping a potentially running container
func (c *RuncContainer) Pid() string {
	return c.pid
}

// State returns the queried state of the container (if available)
func (c *RuncContainer) State() string {
	return c.state
}

//GetPodID return pod-id associated with container.
//only used by CRI-based drivers
func (c *RuncContainer) GetPodID() string {
	return ""
}

// Type returns a driver.Type to indentify the driver implementation
func (r *RuncDriver) Type() Type {
	return Runc
}

// Path returns the binary path of the runc binary in use
func (r *RuncDriver) Path() string {
	return r.runcBinary
}

// Close allows the driver to handle any resource free/connection closing
// as necessary. Runc has no need to perform any actions on close.
func (r *RuncDriver) Close() error {
	return nil
}

// Info returns
func (r *RuncDriver) Info() (string, error) {
	info := "runc driver (binary: " + r.runcBinary + ")\n"
	versionInfo, err := utils.ExecCmd(r.runcBinary, "--v")
	if err != nil {
		return "", fmt.Errorf("Error trying to retrieve runc version info: %v", err)
	}
	return info + versionInfo, nil
}

// Create will create a container instance matching the specific needs
// of a driver
func (r *RuncDriver) Create(name, image, cmdOverride string, detached bool, trace bool) (Container, error) {
	return newRuncContainer(name, image, detached, trace), nil
}

// Clean will clean the environment; removing any remaining containers in the runc metadata
func (r *RuncDriver) Clean() error {
	var tries int
	out, err := utils.ExecCmd(r.runcBinary, "list")
	if err != nil {
		return fmt.Errorf("Error getting runc list output: (err: %v) output: %s", err, out)
	}
	// try up to 3 times to handle any remaining containers in the runc list
	containers := parseRuncList(out)
	log.Infof("Attempting to cleanup runc containers/metadata; %d listed", len(containers))
	for len(containers) > 0 && tries < 3 {
		log.Infof("runc cleanup: Pass #%d", tries+1)
		for _, ctr := range containers {
			switch ctr.State() {
			case "running":
				log.Infof("Attempting stop and remove on container %q", ctr.Name())
				r.Stop(ctr)
				r.Remove(ctr)
			case "paused":
				log.Infof("Attempting unpause and removal of container %q", ctr.Name())
				r.Unpause(ctr)
				r.Remove(ctr)
			case "stopped":
				log.Infof("Attempting remove of container %q", ctr.Name())
				r.Remove(ctr)
			default:
				log.Warnf("Unknown state %q for ctr %q", ctr.State(), ctr.Name())
			}
		}
		tries++
		out, err := utils.ExecCmd(r.runcBinary, "list")
		if err != nil {
			return fmt.Errorf("Error getting runc list output: %v", err)
		}
		containers = parseRuncList(out)
	}
	log.Infof("runc cleanup complete.")
	return nil
}

// Run will execute a container using the driver. Note that if the container is specified to
// run detached, but the config.json for the bundle specifies a "tty" allocation, this
// runc invocation will fail due to the fact we cannot detach without providing a "--console"
// device to runc. Detached daemon/server bundles should not need a tty; stdin/out/err of
// the container will be ignored given this is for benchmarking not validating container
// operation.
func (r *RuncDriver) Run(ctr Container) (string, int, error) {
	var (
		detached string
		trace    string
	)
	if ctr.Detached() {
		detached = "--detach"
	}
	if ctr.Trace() {
		trace = fmt.Sprintf("--trace /tmp/%s.trace ", ctr.Name())
	}

	args := fmt.Sprintf("%srun %s --bundle %s %s", trace, detached, ctr.Image(), ctr.Name())
	// the "NoOut" variant of ExecTimedCmd ignores stdin/out/err (sets them to /dev/null)
	return utils.ExecTimedCmdNoOut(r.runcBinary, args)
}

// Stop will stop/kill a container
func (r *RuncDriver) Stop(ctr Container) (string, int, error) {
	return utils.ExecTimedCmd(r.runcBinary, "kill "+ctr.Name()+" KILL")
}

// Remove will remove a container
func (r *RuncDriver) Remove(ctr Container) (string, int, error) {
	return utils.ExecTimedCmd(r.runcBinary, "delete "+ctr.Name())
}

// Pause will pause a container
func (r *RuncDriver) Pause(ctr Container) (string, int, error) {
	return utils.ExecTimedCmd(r.runcBinary, "pause "+ctr.Name())
}

// Unpause will unpause/resume a container
func (r *RuncDriver) Unpause(ctr Container) (string, int, error) {
	return utils.ExecTimedCmd(r.runcBinary, "resume "+ctr.Name())
}

// take the output of "runc list" and parse into container instances
func parseRuncList(listOutput string) []*RuncContainer {
	var results []*RuncContainer
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
			log.Warnf("runc list parsing found invalid line: %q", line)
			continue
		}
		// don't delete containers that aren't part of our benchmark run!
		if !strings.Contains(parts[0], "bb-") {
			continue
		}
		ctr := &RuncContainer{
			name:       parts[0],
			bundlePath: parts[3],
			pid:        parts[1],
			state:      parts[2],
		}
		results = append(results, ctr)
	}
	return results
}
