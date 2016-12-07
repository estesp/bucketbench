package driver

import (
	"fmt"

	"github.com/estesp/dockerbench/utils"
	log "github.com/sirupsen/logrus"
)

// RuncDriver is an implementation of the driver interface for Runc
type RuncDriver struct {
	runcBinary string
}

// RuncContainer is an implementation of the container metadata needed for runc
type RuncContainer struct {
	name       string
	bundlePath string
	detached   bool
}

// NewRuncDriver creates an instance of the runc driver, providing a path to runc
func NewRuncDriver(binaryPath string) (Driver, error) {
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
func newRuncContainer(name, bundlepath string, detached bool) Container {
	return &RuncContainer{
		name:       name,
		bundlePath: bundlepath,
		detached:   detached,
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

// Image returns the bundle path that runc will use
func (c *RuncContainer) Image() string {
	return c.bundlePath
}

// Type returns a driver.Type to indentify the driver implementation
func (r *RuncDriver) Type() Type {
	return Runc
}

// Info returns
func (r *RuncDriver) Info() string {
	info := "runc driver (binary: " + r.runcBinary + ")\n"
	versionInfo, err := utils.ExecCmd(r.runcBinary, "--v")
	if err != nil {
		log.Warnf("error trying to get runc version info: %v", err)
	} else {
		info = info + versionInfo
	}
	return info
}

// Create will create a container instance matching the specific needs
// of a driver
func (r *RuncDriver) Create(name, image string, detached bool) (Container, error) {
	return newRuncContainer(name, image, detached), nil
}

// Clean will clean the environment; removing any remaining containers in the runc metadata
func (r *RuncDriver) Clean() error {
	return nil
}

// Run will execute a container using the driver
func (r *RuncDriver) Run(ctr Container) (string, int, error) {
	var detached string
	if ctr.Detached() {
		detached = "--detach"
	}
	args := fmt.Sprintf("run %s --bundle %s %s", detached, ctr.Image(), ctr.Name())
	return utils.ExecTimedCmd(r.runcBinary, args)
}

// Stop will stop/kill a container
func (r *RuncDriver) Stop(ctr Container) (string, int, error) {
	return utils.ExecTimedCmd(r.runcBinary, "kill "+ctr.Name())
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
