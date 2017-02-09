package benches

import (
	"fmt"
	"sync"
	"time"

	"github.com/estesp/bucketbench/driver"
	log "github.com/sirupsen/logrus"
)

// BasicBench benchmark runs create, run, remove operations on a simple container
type BasicBench struct {
	driver    driver.Driver
	imageInfo string
	trace     bool
	stats     []RunStatistics
	elapsed   time.Duration
	state     State
	wg        sync.WaitGroup
}

// Init initializes the benchmark
func (bb *BasicBench) Init(driverType driver.Type, binaryPath, imageInfo string, trace bool) error {
	driver, err := driver.New(driverType, binaryPath)
	if err != nil {
		return fmt.Errorf("Error during driver initialization for BasicBench: %v", err)
	}
	// get driver info; will also validate for daemon-based variants whether system is ready/up
	// and running for benchmarking
	info, err := driver.Info()
	if err != nil {
		return fmt.Errorf("Error during driver info query: %v", err)
	}
	log.Infof("Driver initialized: %s", info)
	// prepare environment
	err = driver.Clean()
	if err != nil {
		return fmt.Errorf("Error during driver init cleanup: %v", err)
	}
	bb.imageInfo = imageInfo
	bb.driver = driver
	bb.trace = trace
	return nil
}

//Validate the unit of benchmark execution (create-run-stop-remove)
func (bb *BasicBench) Validate() error {
	ctr, err := bb.driver.Create("bb-test", bb.imageInfo, true, bb.trace)
	if err != nil {
		return fmt.Errorf("Error in Create : %v", err)
	}

	_, _, err = bb.driver.Run(ctr)
	if err != nil {
		return fmt.Errorf("Error in Run : %v", err)
	}

	_, _, err = bb.driver.Stop(ctr)
	if err != nil {
		return fmt.Errorf("Error in Stop : %v", err)
	}
	// allow time for quiesce of stopped state in process and container executor metadata
	time.Sleep(50 * time.Millisecond)

	_, _, err = bb.driver.Remove(ctr)
	if err != nil {
		return fmt.Errorf("Error in Remove : %v", err)
	}
	return nil
}

// Run executes the benchmark iterations against a specific engine driver type
// for a specified number of iterations
func (bb *BasicBench) Run(threads, iterations int) error {
	log.Infof("Start BasicBench run: threads (%d); iterations (%d)", threads, iterations)
	statChan := make([]chan RunStatistics, threads)
	for i := range statChan {
		statChan[i] = make(chan RunStatistics, iterations)
	}
	bb.state = Running
	start := time.Now()
	for i := 0; i < threads; i++ {
		bb.wg.Add(1)
		go bb.runThread(i, iterations, statChan[i])
	}
	bb.wg.Wait()
	bb.elapsed = time.Since(start)

	log.Infof("BasicBench threads complete in %v time elapsed", bb.elapsed)
	//collect stats
	for _, ch := range statChan {
		for statEntry := range ch {
			bb.stats = append(bb.stats, statEntry)
		}
	}
	bb.state = Completed
	return nil
}

func (bb *BasicBench) runThread(threadNum, iterations int, stats chan RunStatistics) {
	for i := 0; i < iterations; i++ {
		var (
			errCount int
			rmErrors bool
		)
		// commands are create, run, remove
		name := fmt.Sprintf("bb-ctr-%d-%d", threadNum, i)
		ctr, err := bb.driver.Create(name, bb.imageInfo, true, bb.trace)

		out, runElapsed, err := bb.driver.Run(ctr)
		if err != nil {
			errCount++
			log.Warnf("BasicBench: run error on container %q: %v\n  Output: %s", name, err, out)
		}

		out, _, err = bb.driver.Stop(ctr)
		if err != nil {
			errCount++
			log.Warnf("BasicBench: stop error on container %q: %v\n  Output: %s", name, err, out)
		}
		// allow time for quiesce of stopped state in process and container executor metadata
		time.Sleep(50 * time.Millisecond)

		out, rmElapsed, err := bb.driver.Remove(ctr)
		if err != nil {
			errCount++
			rmErrors = true
			log.Warnf("BasicBench: remove error on container %q: %v\n  Output: %s", name, err, out)
		}
		stats <- RunStatistics{
			RunDuration:     runElapsed,
			RmDuration:      rmElapsed,
			Errors:          errCount,
			RmFailures:      rmErrors,
			PauseDuration:   -1,
			UnpauseDuration: -1,
		}
	}
	close(stats)
	bb.wg.Done()
}

// Stats returns the statistics of the benchmark run
func (bb *BasicBench) Stats() []RunStatistics {
	if bb.state == Completed {
		return bb.stats
	}
	return []RunStatistics{}
}

// State returns Created, Running, or Completed
func (bb *BasicBench) State() State {
	return bb.state
}

// Elapsed returns the time.Duration taken to run the benchmark
func (bb *BasicBench) Elapsed() time.Duration {
	return bb.elapsed
}

// Type returns the type of benchmark
func (bb *BasicBench) Type() Type {
	return Basic
}
