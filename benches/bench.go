package benches

import (
	"fmt"
	"time"

	"github.com/estesp/bucketbench/driver"
)

// State represents the state of a benchmark object
type State int

// Type represents the type of benchmark
type Type int

// RunStatistics contains perf. data from a run
type RunStatistics struct {
	RunDuration     int
	RmDuration      int
	PauseDuration   int
	UnpauseDuration int
	RmFailures      bool
	Errors          int
}

// State constants
const (
	// Created represents a benchmark not yet run
	Created State = iota
	// Running represents a currently executing benchmark
	Running
	// Completed represents a finished benchmark run
	Completed
)

// Type constants
const (
	// Limit is a benchmark type for testing per-thread limitations
	Limit Type = iota
	// Basic is a simple benchmark for run, kill, remove containers
	Basic
	// Full performs a full sequence of create, run, pause, unpause, kill, remove
	Full
)

// Bench is an interface for a benchmark execution
type Bench interface {

	// Init initializes the benchmark (for example, verifies a daemon is running for daemon-centric
	// engines, pre-pulls images, etc.)
	Init(driverType driver.Type, binaryPath, imageInfo string, trace bool) error

	//Validates the any condition that need to be checked before actual banchmark run.
	//Helpful in testing operations required in benchmark for single run.
	Validate() error

	// Run executes the specified # of iterations against a specified # of
	// threads per benchmark against a specific engine driver type and collects
	// the statistics of each iteration and thread
	Run(threads, iterations int) error

	// Stats returns the statistics of the benchmark run
	Stats() []RunStatistics

	// Elapsed returns the time.Duration that the benchmark took to executes
	Elapsed() time.Duration

	// State returns Created, Running, or Completed
	State() State

	// Type returns the type of benchmark
	Type() Type
}

// New creates an instance of the selected benchmark type
func New(btype Type) (Bench, error) {
	switch btype {
	case Limit:
		return &LimitBench{
			state: Created,
		}, nil
	case Basic:
		return &BasicBench{
			state: Created,
		}, nil
	case Full:
		return nil, fmt.Errorf("full benchmark not implemented")
	default:
		return nil, fmt.Errorf("No such benchmark type: %v", btype)
	}
}
