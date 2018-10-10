package benches

import (
	"context"
	"fmt"
	"time"

	"github.com/estesp/bucketbench/driver"
	"github.com/estesp/bucketbench/stats"
)

// State represents the state of a benchmark object
type State int

// Type represents the type of benchmark
type Type int

// RunStatistics contains performance data from the benchmark run
// Each "step" from the benchmark is named and a map of the name
// to a millisecond duration for that step is provided
type RunStatistics struct {
	Durations map[string]time.Duration
	Errors    map[string]int
	Timestamp time.Time
	Daemon    *stats.ProcMetrics
}

// Benchmark is the object form of a YAML-defined custom benchmark
// used to define the specific operations to perform
type Benchmark struct {
	Name     string
	Image    string
	Command  string // optionally override the default image CMD/ENTRYPOINT
	RootFs   string
	Detached bool
	Drivers  []DriverConfig
	Commands []string
}

// DriverConfig contains the YAML-defined parameters for running a
// benchmark against a specific driver type
type DriverConfig struct {
	Type             string
	ClientPath       string // optional path to specific client binary/socket
	Threads          int
	Iterations       int
	LogDriver        string            `yaml:"logDriver"`
	LogOpts          map[string]string `yaml:"logOpts"`
	CGroupPath       string            `yaml:"cgroupPath"`
	StreamStats      bool              `yaml:"streamStats"`
	StatsIntervalSec int               `yaml:"statsIntervalSec"`
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
	// Limit is a benchmark type for testing per-thread execution limits on the
	// hardware/environment
	Limit Type = iota
	// Custom is a YAML-defined series of container actions run as a benchmark
	Custom
	// Benchmark daemon cpu/memory usage
	Overhead
)

// Bench is an interface to manage benchmark execution against a specific driver
type Bench interface {
	// Init initializes the benchmark (for example, verifies a daemon is running for daemon-centric
	// engines, pre-pulls images, etc.)
	Init(ctx context.Context, name string, driverType driver.Type, binaryPath, imageInfo, cmdOverride string, trace bool) error

	// Validates the any condition that need to be checked before actual banchmark run.
	// Helpful in testing operations required in benchmark for single run.
	Validate(ctx context.Context) error

	// Run executes the specified # of iterations against a specified # of
	// threads per benchmark against a specific engine driver type and collects
	// the statistics of each iteration and thread
	Run(ctx context.Context, threads, iterations int, commands []string) error

	// Stats returns the statistics of the benchmark run
	Stats() []RunStatistics

	// Elapsed returns the time.Duration that the benchmark took to execute
	Elapsed() time.Duration

	// State returns Created, Running, or Completed
	State() State

	// Type returns the type of benchmark
	Type() Type

	// Info returns a string with the driver type and custom benchmark name
	Info(ctx context.Context) (string, error)
}

// New creates an instance of the selected benchmark type
func New(benchType Type, config *DriverConfig) (Bench, error) {
	switch benchType {
	case Limit:
		return &LimitBench{
			state: Created,
		}, nil

	case Custom, Overhead:
		if config.StatsIntervalSec == 0 {
			config.StatsIntervalSec = 1
		}

		statsInterval := time.Duration(config.StatsIntervalSec) * time.Second

		custom := CustomBench{
			state: Created,
			Config: driver.Config{
				LogDriver:     config.LogDriver,
				LogOpts:       config.LogOpts,
				StreamStats:   config.StreamStats,
				StatsInterval: statsInterval,
			},
		}

		if benchType == Custom {
			return &custom, nil
		}

		return &OverheadBench{CustomBench: custom, cgroupPath: config.CGroupPath}, nil
	default:
		return nil, fmt.Errorf("no such benchmark type: %v", benchType)
	}
}

func (b Type) String() string {
	switch b {
	case Limit:
		return "Limit"
	case Custom:
		return "Custom"
	case Overhead:
		return "Overhead"
	default:
		return "Unknown"
	}
}
