package utils

import (
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/process"
)

type Proc struct {
	proc *process.Process
	list []string
}

func NewProcFromPID(pid int, procNames []string) (*Proc, error) {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil, err
	}

	return &Proc{proc: p, list: procNames}, nil
}

// FindPIDByName returns process's PID by its name.
// Note: if there are multiple processes with same name,
// first one will be returned.
func FindPIDByName(name string) (int, error) {
	list, err := process.Processes()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get process list")
	}

	for _, proc := range list {
		procName, err := proc.Name()
		if err != nil {
			return 0, errors.Wrapf(err, "could not get process name for pid %d", proc.Pid)
		}

		if procName == name {
			return int(proc.Pid), nil
		}
	}

	return 0, errors.Errorf("process '%s' not found", name)
}

// PID returns process id
func (p *Proc) PID() int {
	return int(p.proc.Pid)
}

// Mem returns resident memory usage of a process and its children in bytes
func (p *Proc) Mem() (uint64, error) {
	var totalMem uint64
	err := p.walkProcessTree(p.proc, func(p *process.Process) error {
		stat, err := p.MemoryInfo()
		if err != nil {
			return err
		}

		totalMem += stat.RSS
		return nil
	})

	return totalMem, err
}

// CPU returns how many percents of the CPU a process and its children use between this and previous call
func (p *Proc) CPU() (float64, error) {
	var totalCPU float64
	err := p.walkProcessTree(p.proc, func(p *process.Process) error {
		cpu, err := p.Percent(0)
		if err != nil {
			return err
		}

		totalCPU += cpu
		return nil
	})

	return totalCPU, err
}

func (p *Proc) walkProcessTree(root *process.Process, callback func(*process.Process) error) error {
	rootName, err := root.Name()
	if err != nil {
		// If no /proc/pid/stat file, process gone
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	// Check this process is in the whitelist
	found := false
	for _, procName := range p.list {
		if strings.EqualFold(rootName, procName) {
			found = true
			break
		}
	}

	if !found {
		return nil
	}

	if err := callback(root); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return errors.Wrapf(err, "couldn't query process '%s' (pid: %d)", rootName, root.Pid)
	}

	children, err := root.Children()
	if err != nil {
		// Children returns an error if no child processes
		return nil
	}

	for _, child := range children {
		if err := p.walkProcessTree(child, callback); err != nil {
			return err
		}
	}

	return nil
}
