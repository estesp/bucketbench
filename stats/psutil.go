package stats

import (
	"github.com/estesp/bucketbench/utils"
	"github.com/pkg/errors"
)

const bytesInMiB = 1024 * 1024

type PSUtilSampler struct {
	proc *utils.Proc
}

func NewPSUtilSampler(proc Process) (*PSUtilSampler, error) {
	pid, err := proc.PID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get root process pid")
	}

	daemonProc, err := utils.NewProcFromPID(pid, proc.ProcNames())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create process from pid: %d", pid)
	}

	return &PSUtilSampler{daemonProc}, nil
}

func (s *PSUtilSampler) Query() (*ProcMetrics, error) {
	mem, memErr := s.proc.Mem()
	if memErr != nil {
		return nil, errors.Wrapf(memErr, "couldn't get mem info for proc: %d", s.proc.PID())
	}

	cpu, cpuErr := s.proc.CPU()
	if cpuErr != nil {
		return nil, errors.Wrapf(cpuErr, "couldn't get cpu info for proc: %d", s.proc.PID())
	}

	return &ProcMetrics{
		Mem: mem / bytesInMiB,
		CPU: cpu,
	}, nil
}

func (s *PSUtilSampler) Close() error {
	return nil
}