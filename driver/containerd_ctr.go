package driver

import (
	"bufio"
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/estesp/bucketbench/utils"
)

const defaultCtrBinary = "ctr"

// CtrDriver is an implementation of the driver interface for using Containerd
// in "legacy" mode via the `ctr` client binary (meant for testing purposes only).
// IMPORTANT: This implementation does not protect instance metadata for thread safely.
// At this time there is no understood use case for multi-threaded use of this implementation.
type CtrDriver struct {
	ctrBinary string
}

// CtrContainer is an implementation of the container metadata needed for containerd
type CtrContainer struct {
	name       string
	bundlePath string
	state      string
	process    string
	trace      bool
}

// NewCtrDriver creates an instance of the containerd driver, providing a path to the ctr client
func NewCtrDriver(binaryPath string) (Driver, error) {
	if binaryPath == "" {
		binaryPath = defaultCtrBinary
	}
	resolvedBinPath, err := utils.ResolveBinary(binaryPath)
	if err != nil {
		return &CtrDriver{}, err
	}
	driver := &CtrDriver{
		ctrBinary: resolvedBinPath,
	}
	return driver, nil
}

// newContainerdContainer creates the metadata object of a containerd-specific container with
// bundle, name, and any required additional information
func newCtrContainer(name, bundlepath string, trace bool) Container {
	return &CtrContainer{
		name:       name,
		bundlePath: bundlepath,
		trace:      trace,
	}
}

// Name returns the name of the container
func (c *CtrContainer) Name() string {
	return c.name
}

// Trace returns whether the container should be started with tracing enabled
func (c *CtrContainer) Trace() bool {
	return c.trace
}

// Image returns the bundle path that runc will use
func (c *CtrContainer) Image() string {
	return c.bundlePath
}

// Process returns the process name in cases where this container instance is
// wrapping a potentially running container
func (c *CtrContainer) Process() string {
	return c.process
}

// State returns the queried state of the container (if available)
func (c *CtrContainer) State() string {
	return c.state
}

// Detached always returns true for containerd as IO streams are always detached
func (c *CtrContainer) Detached() bool {
	return true
}

// Type returns a driver.Type to indentify the driver implementation
func (r *CtrDriver) Type() Type {
	return Ctr
}

// Info returns
func (r *CtrDriver) Info() (string, error) {
	info := "containerd legacy driver (ctr client binary: " + r.ctrBinary + ")"
	clientVersionInfo, err := utils.ExecCmd(r.ctrBinary, "--v")
	if err != nil {
		return "", fmt.Errorf("Error trying to retrieve containerd client version info: %v", err)
	}
	daemonVersionInfo, err := utils.ExecCmd(r.ctrBinary, "version")
	if err != nil {
		return "", fmt.Errorf("Error trying to retrieve containerd daemon version info: %v", err)
	}
	fullInfo := fmt.Sprintf("%s[version: %s][daemon version: %s]", info,
		strings.TrimSpace(clientVersionInfo), strings.TrimSpace(daemonVersionInfo))
	return fullInfo, nil
}

// Create will create a container instance matching the specific needs
// of a driver
func (r *CtrDriver) Create(name, image string, detached bool, trace bool) (Container, error) {
	return newCtrContainer(name, image, trace), nil
}

// Clean will clean the environment; removing any remaining containers in the runc metadata
func (r *CtrDriver) Clean() error {
	var tries int
	out, err := utils.ExecCmd(r.ctrBinary, "containers")
	if err != nil {
		return fmt.Errorf("Error getting containerd list output: (err: %v) output: %s", err, out)
	}
	// try up to 3 times to handle any remaining containers in the runc list
	containers := parseContainerdList(out)
	log.Infof("Attempting to cleanup containerd containers/metadata; %d listed", len(containers))
	for len(containers) > 0 && tries < 3 {
		log.Infof("containerd cleanup: Pass #%d", tries+1)
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
		out, err := utils.ExecCmd(r.ctrBinary, "containers")
		if err != nil {
			return fmt.Errorf("Error getting containerd list output: %v", err)
		}
		containers = parseContainerdList(out)
	}
	log.Infof("containerd cleanup complete.")
	return nil
}

// Run will execute a container using the containerd driver.
func (r *CtrDriver) Run(ctr Container) (string, int, error) {
	args := fmt.Sprintf("containers start %s %s", ctr.Name(), ctr.Image())
	// the "NoOut" variant of ExecTimedCmd ignores stdin/out/err (sets them to /dev/null)
	return utils.ExecTimedCmdNoOut(r.ctrBinary, args)
}

// Stop will stop/kill a container
func (r *CtrDriver) Stop(ctr Container) (string, int, error) {
	return utils.ExecTimedCmd(r.ctrBinary, "containers kill "+ctr.Name())
}

// Remove will remove a container; in the containerd case we simply call kill
// which will remove any container metadata if it was running
func (r *CtrDriver) Remove(ctr Container) (string, int, error) {
	return utils.ExecTimedCmd(r.ctrBinary, "containers kill "+ctr.Name())
}

// Pause will pause a container
func (r *CtrDriver) Pause(ctr Container) (string, int, error) {
	return utils.ExecTimedCmd(r.ctrBinary, "containers pause "+ctr.Name())
}

// Unpause will unpause/resume a container
func (r *CtrDriver) Unpause(ctr Container) (string, int, error) {
	return utils.ExecTimedCmd(r.ctrBinary, "containers resume "+ctr.Name())
}

// take the output of "runc list" and parse into container instances
func parseContainerdList(listOutput string) []*CtrContainer {
	var results []*CtrContainer
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
			log.Warnf("containerd list parsing found invalid line: %q", line)
			continue
		}
		ctr := &CtrContainer{
			name:       parts[0],
			bundlePath: parts[1],
			process:    parts[3],
			state:      parts[2],
		}
		results = append(results, ctr)
	}
	return results
}
