package benches

import (
	"context"
	"sort"
	"time"

	"github.com/estesp/bucketbench/stats"
	log "github.com/sirupsen/logrus"
)

const (
	procMetricsSampleInterval = 500 * time.Millisecond
)

// OverheadBench runs CustomBench benchmarks and measure memory and cpu usage of a container daemon
type OverheadBench struct {
	*CustomBench
	cgroupPath string
}

// Run executes the benchmark iterations against a specific engine driver type
// for a specified number of iterations
func (b *OverheadBench) Run(ctx context.Context, threads, iterations int, commands []string) error {
	sampler, err := stats.NewSampler(b.driver, b.cgroupPath)
	if err != nil {
		log.WithError(err).Error("failed to create stats sampler")
		return err
	}

	var metrics []RunStatistics
	ticker := time.NewTicker(procMetricsSampleInterval)

	go func() {
		for range ticker.C {
			result, err := sampler.Query()
			if err != nil {
				log.WithError(err).Error("stats sample failed")
				continue
			}

			stat := RunStatistics{
				Timestamp: time.Now().UTC(),
				Daemon:    result,
			}

			metrics = append(metrics, stat)
		}
	}()

	err = b.CustomBench.Run(ctx, threads, iterations, commands)

	// Stop gathering metrics
	ticker.Stop()

	b.stats = append(b.stats, metrics...)
	sort.Slice(b.stats, func(i, j int) bool {
		return b.stats[i].Timestamp.Before(b.stats[j].Timestamp)
	})

	return err
}
