//go:build windows
// +build windows

package stats

import (
	"time"

	"github.com/pkg/errors"
)

// CGroupsSampler is a stub on Windows
type CGroupsSampler struct {
	lastCPUUsage uint64
	lastCPUTime  time.Time
}

// NewCGroupsSampler creates a stats sampler from existing control group
func NewCGroupsSampler(path string) (*CGroupsSampler, error) {
	return nil, errors.New("unimplemented")
}

// Query gets a process metrics from control cgroup
func (s *CGroupsSampler) Query() (*ProcMetrics, error) {
	return nil, errors.New("unimplemented")
}
