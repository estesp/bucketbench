package benches

import (
	"context"
	"sync"
	"time"

	"github.com/estesp/bucketbench/driver"
	"github.com/estesp/bucketbench/utils"
	log "github.com/sirupsen/logrus"
)

// LimitBench only checks per-thread throughput as a baseline for comparison to runs on
// other hardware/environments.
// IMPORTANT: This implementation does not protect instance metadata for thread safely.
// At this time there is no understood use case for multi-threaded use of this implementation.
type LimitBench struct {
	stats   []RunStatistics
	elapsed time.Duration
	state   State
	wg      sync.WaitGroup
}

// Init initializes the benchmark
func (lb *LimitBench) Init(ctx context.Context, name string, driverType driver.Type, binaryPath, imageInfo, cmdOverride string, trace bool) error {
	return nil
}

// Validate the unit of benchmark execution
func (lb *LimitBench) Validate(ctx context.Context) error {
	return nil
}

// Run executes the benchmark iterations against a specific engine driver type
// for a specified number of iterations
func (lb *LimitBench) Run(ctx context.Context, threads, iterations int, commands []string) error {
	log.Infof("Start LimitBench run: threads (%d); iterations (%d)", threads, iterations)
	statChan := make([]chan RunStatistics, threads)
	for i := range statChan {
		statChan[i] = make(chan RunStatistics, iterations)
	}
	lb.state = Running
	start := time.Now()
	for i := 0; i < threads; i++ {
		lb.wg.Add(1)
		go lb.runThread(ctx, iterations, statChan[i])
	}
	lb.wg.Wait()
	lb.elapsed = time.Since(start)

	log.Infof("LimitBench threads complete in %v time elapsed", lb.elapsed)
	//collect stats
	for _, ch := range statChan {
		for statEntry := range ch {
			lb.stats = append(lb.stats, statEntry)
		}
	}
	lb.state = Completed
	return nil
}

func (lb *LimitBench) runThread(ctx context.Context, iterations int, stats chan RunStatistics) {
	for i := 0; i < iterations; i++ {
		_, elapsed, _ := utils.ExecTimedCmd(ctx, "ls", "/tmp")
		stats <- RunStatistics{
			Durations: map[string]time.Duration{"run": elapsed},
		}
	}
	close(stats)
	lb.wg.Done()
}

// Stats returns the statistics of the benchmark run
func (lb *LimitBench) Stats() []RunStatistics {
	if lb.state == Completed {
		return lb.stats
	}
	return []RunStatistics{}
}

// State returns Created, Running, or Completed
func (lb *LimitBench) State() State {
	return lb.state
}

// Elapsed returns the time.Duration taken to run the benchmark
func (lb *LimitBench) Elapsed() time.Duration {
	return lb.elapsed
}

// Type returns the type of benchmark
func (lb *LimitBench) Type() Type {
	return Limit
}

// Info returns a string with the driver type and custom benchmark name
func (lb *LimitBench) Info(ctx context.Context) (string, error) {
	return "Limit benchmark: No driver", nil
}
