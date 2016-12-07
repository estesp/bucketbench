// Copyright © 2016 Phil Estes <estesp@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"

	"github.com/estesp/dockerbench/benches"
	"github.com/estesp/dockerbench/driver"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	defaultLimitThreads      = 10
	defaultLimitIter         = 1000
	defaultDockerThreads     = 0
	defaultRuncThreads       = 0
	defaultContainerdThreads = 0
	defaultDockerBinary      = "docker"
	defaultRuncBinary        = "runc"
	defaultContainerdBinary  = "ctr"
	defaultDockerImage       = "busybox"
	defaultRuncBundle        = "."
)

var (
	dockerThreads     int
	runcThreads       int
	containerdThreads int
	dockerBinary      string
	runcBinary        string
	containerdBinary  string
	dockerImage       string
	runcBundle        string
)
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the benchmarks against the selected container engine components",
	Long: `Providing the number of threads selected for each possible engine, this
command will run those number of threads with the pre-defined lifecycle commands
and then report the results to the terminal.`,
	RunE: func(cmd *cobra.Command, args []string) error {

		var rates []float64
		// get thread limit stats
		for i := 1; i <= defaultLimitThreads; i++ {
			limit, _ := benches.New(benches.Limit)
			limit.Init(driver.Null, "", "")
			limit.Run(i, defaultLimitIter)
			duration := limit.Elapsed()
			rate := float64(i*defaultLimitIter) / duration.Seconds()
			rates = append(rates, rate)
			log.Infof("threads %d, iterations %d, rate: %6.2f", i, defaultLimitIter, rate)
		}
		fmt.Printf("             Iter/Thd    1 thrd ")
		for i := 2; i <= defaultLimitThreads; i++ {
			fmt.Printf(" %2d thrds", i)
		}
		fmt.Printf("\n%-13s   %5d    %6.2f", "Limit", defaultLimitIter, rates[0])
		for i := 1; i < defaultLimitThreads; i++ {
			fmt.Printf("  %6.2f", rates[i])
		}
		fmt.Printf("\n\n")

		log.Info("Benchmark runs complete")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(runCmd)
	runCmd.PersistentFlags().IntVarP(&dockerThreads, "docker", "d", defaultDockerThreads, "Number of threads to execute against Docker")
	runCmd.PersistentFlags().IntVarP(&runcThreads, "runc", "r", defaultRuncThreads, "Number of threads to execute against runc")
	runCmd.PersistentFlags().IntVarP(&containerdThreads, "containerd", "c", defaultContainerdThreads, "Number of threads to execute against containerd")
	runCmd.PersistentFlags().StringVarP(&dockerBinary, "docker-binary", "", defaultDockerBinary, "Name/path of Docker binary")
	runCmd.PersistentFlags().StringVarP(&runcBinary, "runc-binary", "", defaultRuncBinary, "Name/path of runc binary")
	runCmd.PersistentFlags().StringVarP(&containerdBinary, "ctr-binary", "", defaultContainerdBinary, "Name/path of containerd client (ctr) binary")
	runCmd.PersistentFlags().StringVarP(&dockerImage, "image", "i", defaultDockerImage, "Name of test Docker image")
	runCmd.PersistentFlags().StringVarP(&runcBundle, "bundle", "b", defaultRuncBundle, "Path of test runc image bundle")
}
