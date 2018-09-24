package benches

import (
	"sort"
	"time"

	"github.com/estesp/bucketbench/utils"
	log "github.com/sirupsen/logrus"
)

type OverheadBench struct {
	CustomBench
}

func (b *OverheadBench) Run(threads, iterations int, commands []string) error {
	pid, err := b.driver.PID()
	if err != nil {
		log.WithError(err).Errorf("could not find daemon with pid: %d", pid)
		return err
	}

	log.Infof("daemod pid: %d", pid)
	daemonProc, err := utils.NewProcFromPID(pid)
	if err != nil {
		log.WithError(err).Error("could not get proc info: %v", err)
		return err
	}

	var metrics []RunStatistics

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)

		for {
			select {
			case <-ticker.C:
				mem, err := daemonProc.Mem()
				if err != nil {
					log.WithError(err).Error("could not get memory info")
				}

				cpu, err := daemonProc.CPU()
				if err != nil {
					log.WithError(err).Error("could not get cpu info")
				}

				stat := RunStatistics{
					Timestamp: time.Now().UTC(),
					Daemon:    &ProcMetrics{Mem: mem / 1024 / 1024, CPU: cpu},
				}

				metrics = append(metrics, stat)

			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	err = b.CustomBench.Run(threads, iterations, commands)

	// Stop gathering metrics
	close(done)

	b.stats = append(b.stats, metrics...)
	sort.Slice(b.stats, func(i, j int) bool {
		return b.stats[i].Timestamp.Before(b.stats[j].Timestamp)
	})

	return err
}
