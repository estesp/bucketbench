package stats

import (
	"runtime"
)

// ProcMetrics represents stats sample from daemon
type ProcMetrics struct {
	Mem uint64
	CPU float64
}

// Process represents an interfaces of a daemon to be sampled
type Process interface {
	// PID returns daemon process id
	PID() (int, error)

	// ProcNames returns the list of process names contributing to mem/cpu usage during overhead benchmark
	ProcNames() []string
}

// Sampler represents an interface of a sampler
type Sampler interface {
	// Query gets a process metrics (cpu and memory usage) or error
	Query() (*ProcMetrics, error)

	// Close clean things up after sampler is done
	Close() error
}

// NewSampler creates a CGroups stats sampler on Linux and fallbacks to psutils implementation on
// other operating systems
func NewSampler(proc Process) (Sampler, error) {
	if runtime.GOOS == "linux" {
		return NewCGroupsSampler()
	} else {
		return NewPSUtilSampler(proc)
	}
}
