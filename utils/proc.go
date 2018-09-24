package utils

import (
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/process"
)

type Proc struct {
	proc *process.Process
}

func NewProcFromPID(pid int) (*Proc, error) {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil, err
	}

	return &Proc{p}, nil
}

func NewProcFromName(name string) (*Proc, error) {
	list, err := process.Processes()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get process list")
	}

	for _, proc := range list {
		procName, err := proc.Name()
		if err != nil {
			return nil, errors.Wrapf(err, "could not get process name for pid %d", proc.Pid)
		}

		if procName == name {
			return NewProcFromPID(int(proc.Pid))
		}
	}

	return nil, errors.Errorf("process '%s' not found", name)
}

// PID returns process id
func (p *Proc) PID() int {
	return int(p.proc.Pid)
}

// Mem returns resident memory usage in bytes
func (p *Proc) Mem() (uint64, error) {
	stat, err := p.proc.MemoryInfo()
	if err != nil {
		return 0, err
	}

	return stat.RSS, nil
}

// CPU returns how many percents of the CPU a process uses between this and previous call
func (p *Proc) CPU() (float64, error) {
	return p.proc.Percent(0)
}
