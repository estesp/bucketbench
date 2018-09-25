package benches

import (
	"sort"
	"time"

	"github.com/estesp/bucketbench/utils"
	log "github.com/sirupsen/logrus"
)

const procMetricsSampleInterval = 500 * time.Millisecond

type OverheadBench struct {
	CustomBench
}

func (b *OverheadBench) Run(threads, iterations int, commands []string) error {
	pid, err := b.driver.PID()
	if err != nil {
		log.WithError(err).Errorf("could not find daemon with pid: %d", pid)
		return err
	}

	log.Infof("daemon pid: %d", pid)
	daemonProc, err := utils.NewProcFromPID(pid)
	if err != nil {
		log.WithError(err).Error("could not get proc info: %v", err)
		return err
	}

	var metrics []RunStatistics

	ticker := time.NewTicker(procMetricsSampleInterval)

	go func() {
		for range ticker.C {
			mem, err1 := daemonProc.Mem()
			if err1 != nil {
				log.WithError(err).Error("could not get memory info")
			}

			cpu, err2 := daemonProc.CPU()
			if err2 != nil {
				log.WithError(err).Error("could not get cpu info")
			}

			if err1 == nil || err2 == nil {
				stat := RunStatistics{
					Timestamp: time.Now().UTC(),
					Daemon:    &ProcMetrics{Mem: mem / 1024 / 1024, CPU: cpu},
				}

				metrics = append(metrics, stat)
			}
		}
	}()

	err = b.CustomBench.Run(threads, iterations, commands)

	// Stop gathering metrics
	ticker.Stop()

	b.stats = append(b.stats, metrics...)
	sort.Slice(b.stats, func(i, j int) bool {
		return b.stats[i].Timestamp.Before(b.stats[j].Timestamp)
	})

	return err
}
