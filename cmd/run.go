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

	dockerIter = 15
	runcIter   = 50
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

		var (
			err         error
			limitRates  []float64
			dockerRates []float64
			runcRates   []float64
			//ctrRates    []float64
		)
		// get thread limit stats
		limitRates = runLimitTest()

		if dockerThreads > 0 {
			// run basic benchmark against Docker
			dockerRates, err = runDockerBasicBench()
			if err != nil {
				log.Errorf("Error during docker basic benchmark execution: %v", err)
				return err
			}
		}
		if runcThreads > 0 {
			// run basic benchmark against runc
			runcRates, err = runRuncBasicBench()
			if err != nil {
				log.Errorf("Error during runc basic benchmark execution: %v", err)
				return err
			}
		}
		if containerdThreads > 0 {
			// run basic benchmark against containerd
		}

		// output benchmark results
		outputRunDetails(limitRates, dockerRates, runcRates)

		log.Info("Benchmark runs complete")
		return nil
	},
}

func runLimitTest() []float64 {
	var rates []float64
	// get thread limit stats
	for i := 1; i <= defaultLimitThreads; i++ {
		limit, _ := benches.New(benches.Limit)
		limit.Init(driver.Null, "", "")
		limit.Run(i, defaultLimitIter)
		duration := limit.Elapsed()
		rate := float64(i*defaultLimitIter) / duration.Seconds()
		rates = append(rates, rate)
		log.Infof("Limit: threads %d, iterations %d, rate: %6.2f", i, defaultLimitIter, rate)
	}
	return rates
}

func runDockerBasicBench() ([]float64, error) {
	var rates []float64
	for i := 1; i <= dockerThreads; i++ {
		basic, _ := benches.New(benches.Basic)
		err := basic.Init(driver.Docker, dockerBinary, dockerImage)
		if err != nil {
			return []float64{}, err
		}
		err = basic.Run(i, dockerIter)
		if err != nil {
			return []float64{}, fmt.Errorf("Error during basic bench run: %v", err)
		}
		duration := basic.Elapsed()
		rate := float64(i*dockerIter) / duration.Seconds()
		rates = append(rates, rate)
		log.Infof("Docker Basic: threads %d, iterations %d, rate: %6.2f", i, dockerIter, rate)
	}
	return rates, nil
}

func runRuncBasicBench() ([]float64, error) {
	var rates []float64
	for i := 1; i <= runcThreads; i++ {
		basic, _ := benches.New(benches.Basic)
		err := basic.Init(driver.Runc, runcBinary, runcBundle)
		if err != nil {
			return []float64{}, err
		}
		err = basic.Run(i, runcIter)
		if err != nil {
			return []float64{}, fmt.Errorf("Error during basic bench run: %v", err)
		}
		duration := basic.Elapsed()
		rate := float64(i*runcIter) / duration.Seconds()
		rates = append(rates, rate)
		log.Infof("Runc Basic: threads %d, iterations %d, rate: %6.2f", i, runcIter, rate)
	}
	return rates, nil
}

func outputRunDetails(limitRates, dockerRates, runcRates []float64) {
	fmt.Printf("             Iter/Thd    1 thrd ")
	for i := 2; i <= defaultLimitThreads; i++ {
		fmt.Printf(" %2d thrds", i)
	}
	fmt.Printf("\n%-13s   %5d    %6.2f", "Limit", defaultLimitIter, limitRates[0])
	for i := 1; i < defaultLimitThreads; i++ {
		fmt.Printf("  %6.2f", limitRates[i])
	}
	if dockerThreads > 0 {
		fmt.Printf("\n%-13s   %5d    %6.2f", "DockerBasic", dockerIter, dockerRates[0])
		for i := 1; i < dockerThreads; i++ {
			fmt.Printf("  %6.2f", dockerRates[i])
		}
	}
	if runcThreads > 0 {
		fmt.Printf("\n%-13s   %5d    %6.2f", "RuncBasic", runcIter, runcRates[0])
		for i := 1; i < runcThreads; i++ {
			fmt.Printf("  %6.2f", runcRates[i])
		}
	}
	fmt.Printf("\n\n")
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
