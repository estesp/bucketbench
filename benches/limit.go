package benches

import (
	"sync"
	"time"

	"github.com/estesp/dockerbench/driver"
	"github.com/estesp/dockerbench/utils"
	log "github.com/sirupsen/logrus"
)

// LimitBench only checks per-thread throughput as a baseline for other runs
type LimitBench struct {
	stats   []RunStatistics
	elapsed time.Duration
	state   State
	wg      sync.WaitGroup
}

// Init initializes the benchmark
func (lb *LimitBench) Init(driverType driver.Type, binaryPath, imageInfo string) error {
	return nil
}

// Run executes the benchmark iterations against a specific engine driver type
// for a specified number of iterations
func (lb *LimitBench) Run(threads, iterations int) error {
	log.Infof("Start LimitBench run: threads (%d); iterations (%d)", threads, iterations)
	statChan := make([]chan RunStatistics, threads)
	for i := range statChan {
		statChan[i] = make(chan RunStatistics, iterations)
	}
	lb.state = Running
	start := time.Now()
	for i := 0; i < threads; i++ {
		lb.wg.Add(1)
		go lb.runThread(iterations, statChan[i])
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

func (lb *LimitBench) runThread(iterations int, stats chan RunStatistics) {
	for i := 0; i < iterations; i++ {
		_, elapsed, _ := utils.ExecTimedCmd("ls", "/tmp")
		//_, elapsed, _ := utils.ExecTimedCmd("date", "")
		stats <- RunStatistics{
			RunDuration:     elapsed,
			RmDuration:      -1,
			PauseDuration:   -1,
			UnpauseDuration: -1,
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
