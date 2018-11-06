package stats

import (
	"time"

	"github.com/containerd/cgroups"
	"github.com/pkg/errors"
)

// CGroupsSampler represents Linux cgroups sampler
type CGroupsSampler struct {
	control      cgroups.Cgroup
	lastCPUUsage uint64
	lastCPUTime  time.Time
}

// NewCGroupsSampler creates a stats sampler from existing control group
func NewCGroupsSampler(path string) (*CGroupsSampler, error) {
	control, err := cgroups.Load(reportControllers, cgroups.StaticPath(path))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load cgroup: '%s'", path)
	}

	return &CGroupsSampler{control: control}, nil
}

// reportControllers returns v1 controllers only required for measuring resource usage
func reportControllers() ([]cgroups.Subsystem, error) {
	v1, err := cgroups.V1()
	if err != nil {
		return nil, err
	}

	var out []cgroups.Subsystem
	for _, sub := range v1 {
		if sub.Name() == cgroups.Memory || sub.Name() == cgroups.Cpuacct {
			out = append(out, sub)
		}
	}

	return out, nil
}

// Query gets a process metrics from control cgroup
func (s *CGroupsSampler) Query() (*ProcMetrics, error) {
	metrics, err := s.control.Stat(cgroups.IgnoreNotExist)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get metrics from cgroup")
	}

	memStat := metrics.Memory

	// memory.memsw.usage_in_bytes (current usage for memory+swap) + memory.kmem.usage_in_bytes (current
	// kernel memory allocation)
	mem := (memStat.Swap.Usage + memStat.Kernel.Usage) / bytesInMiB
	cpu := metrics.CPU.Usage.Total

	now := time.Now()

	cpuUsage := float64(cpu-s.lastCPUUsage) / float64(now.Sub(s.lastCPUTime).Nanoseconds())

	s.lastCPUUsage = cpu
	s.lastCPUTime = now

	return &ProcMetrics{
		Mem: mem,
		CPU: cpuUsage,
	}, nil
}
