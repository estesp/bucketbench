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

	log "github.com/Sirupsen/logrus"
	"github.com/estesp/bucketbench/benches"
	"github.com/estesp/bucketbench/driver"
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

	dockerIter     = 15
	runcIter       = 50
	containerdIter = 50
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
	trace             bool
)

// simple structure to handle collecting output data which will be displayed
// after all benchmarks are complete
type benchResult struct {
	name        string
	threads     int
	iterations  int
	threadRates []float64
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the benchmarks against the selected container engine components",
	Long: `Providing the number of threads selected for each possible engine, this
command will run those number of threads with the pre-defined lifecycle commands
and then report the results to the terminal.`,
	RunE: func(cmd *cobra.Command, args []string) error {

		var (
			maxThreads = defaultLimitThreads
			results    []benchResult
		)
		// get thread limit stats
		limitRates := runLimitTest()
		limitResult := benchResult{
			name:        "Limit",
			threads:     defaultLimitThreads,
			iterations:  defaultLimitIter,
			threadRates: limitRates,
		}
		results = append(results, limitResult)

		if dockerThreads > 0 {
			// run basic benchmark against Docker
			dockerRates, err := runDockerBasicBench()
			if err != nil {
				log.Errorf("Error during docker basic benchmark execution: %v", err)
				return err
			}
			dockerResult := benchResult{
				name:        "DockerBasic",
				threads:     dockerThreads,
				iterations:  dockerIter,
				threadRates: dockerRates,
			}
			results = append(results, dockerResult)
			maxThreads = intMax(maxThreads, dockerThreads)
		}
		if runcThreads > 0 {
			// run basic benchmark against runc
			runcRates, err := runRuncBasicBench()
			if err != nil {
				log.Errorf("Error during runc basic benchmark execution: %v", err)
				return err
			}
			runcResult := benchResult{
				name:        "RuncBasic",
				threads:     runcThreads,
				iterations:  runcIter,
				threadRates: runcRates,
			}
			results = append(results, runcResult)
			maxThreads = intMax(maxThreads, runcThreads)
		}
		if containerdThreads > 0 {
			// run basic benchmark against containerd
			containerdRates, err := runContainerdBasicBench()
			if err != nil {
				log.Errorf("Error during containerd basic benchmark execution: %v", err)
				return err
			}
			containerdResult := benchResult{
				name:        "ContainerdBasic",
				threads:     containerdThreads,
				iterations:  containerdIter,
				threadRates: containerdRates,
			}
			results = append(results, containerdResult)
			maxThreads = intMax(maxThreads, containerdThreads)
		}

		// output benchmark results
		outputRunDetails(maxThreads, results)

		log.Info("Benchmark runs complete")
		return nil
	},
}

func runLimitTest() []float64 {
	var rates []float64
	// get thread limit stats
	for i := 1; i <= defaultLimitThreads; i++ {
		limit, _ := benches.New(benches.Limit)
		limit.Init(driver.Null, "", "", trace)
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
		err := basic.Init(driver.Docker, dockerBinary, dockerImage, trace)
		if err != nil {
			return []float64{}, err
		}

		if err = basic.Validate(); err != nil {
			return []float64{}, fmt.Errorf("Error during basic bench validate: %v", err)
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
		err := basic.Init(driver.Runc, runcBinary, runcBundle, trace)
		if err != nil {
			return []float64{}, err
		}

		if err = basic.Validate(); err != nil {
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

func runContainerdBasicBench() ([]float64, error) {
	var rates []float64
	for i := 1; i <= containerdThreads; i++ {
		basic, _ := benches.New(benches.Basic)
		err := basic.Init(driver.Containerd, containerdBinary, runcBundle, trace)
		if err != nil {
			return []float64{}, err
		}

		if err = basic.Validate(); err != nil {
			return []float64{}, err
		}

		err = basic.Run(i, containerdIter)
		if err != nil {
			return []float64{}, fmt.Errorf("Error during basic bench run: %v", err)
		}
		duration := basic.Elapsed()
		rate := float64(i*runcIter) / duration.Seconds()
		rates = append(rates, rate)
		log.Infof("Containerd Basic: threads %d, iterations %d, rate: %6.2f", i, containerdIter, rate)
	}
	return rates, nil
}

func outputRunDetails(maxThreads int, results []benchResult) {
	fmt.Printf("             Iter/Thd     1 thrd")
	for i := 2; i <= maxThreads; i++ {
		fmt.Printf(" %2d thrds", i)
	}
	for _, result := range results {
		fmt.Printf("\n%-15s %5d    %7.2f", result.name, result.iterations, result.threadRates[0])
		for i := 1; i < result.threads; i++ {
			fmt.Printf("  %7.2f", result.threadRates[i])
		}
	}
	fmt.Printf("\n\n")
}

func intMax(x, y int) int {
	if x > y {
		return x
	}
	return y
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
	runCmd.PersistentFlags().BoolVarP(&trace, "trace", "t", false, "Enable per-container tracing during benchmark runs")
}
