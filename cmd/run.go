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
	"io/ioutil"

	log "github.com/Sirupsen/logrus"
	"github.com/estesp/bucketbench/benches"
	"github.com/estesp/bucketbench/driver"
	"github.com/go-yaml/yaml"
	"github.com/spf13/cobra"
)

const (
	defaultLimitThreads = 10
	defaultLimitIter    = 1000
)

var (
	yamlFile  string
	trace     bool
	skipLimit bool
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
	Short: "Run the benchmark against the selected container engine components",
	Long: `The YAML file provided via the --benchmark flag will determine which
lifecycle container commands to run against which container runtimes, specifying
iterations and number of concurrent threads. Results will be displayed afterwards.`,
	RunE: func(cmd *cobra.Command, args []string) error {

		if yamlFile == "" {
			return fmt.Errorf("No YAML file provided with --benchmark/-b; nothing to do")
		}
		benchmark, err := readYaml(yamlFile)
		if err != nil {
			return fmt.Errorf("Error reading benchmark file %q: %v", yamlFile, err)
		}
		// verify that an image name exists in the benchmark as
		// we'll end up erroring out further down if no image is
		// specified
		if benchmark.Image == "" {
			return fmt.Errorf("Please provide an 'image:' entry in your benchmark YAML")
		}

		var (
			maxThreads = defaultLimitThreads
			results    []benchResult
		)
		if !skipLimit {
			// get thread limit stats
			limitRates := runLimitTest()
			limitResult := benchResult{
				name:        "Limit",
				threads:     defaultLimitThreads,
				iterations:  defaultLimitIter,
				threadRates: limitRates,
			}
			results = append(results, limitResult)
		} else {
			maxThreads = 0 // no limit results in output
		}

		for _, driverEntry := range benchmark.Drivers {
			result, err := runBenchmark(driverEntry, benchmark)
			if err != nil {
				return err
			}
			results = append(results, result)
			maxThreads = intMax(maxThreads, driverEntry.Threads)
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
		limit.Init("", driver.Null, "", "", trace)
		limit.Run(i, defaultLimitIter, nil)
		duration := limit.Elapsed()
		rate := float64(i*defaultLimitIter) / duration.Seconds()
		rates = append(rates, rate)
		log.Infof("Limit: threads %d, iterations %d, rate: %6.2f", i, defaultLimitIter, rate)
	}
	return rates
}

func runBenchmark(driverConfig benches.DriverConfig, benchmark benches.Benchmark) (benchResult, error) {
	var (
		rates     []float64
		benchInfo string
	)
	driverType := driver.StringToType(driverConfig.Type)

	for i := 1; i <= driverConfig.Threads; i++ {
		bench, _ := benches.New(benches.Custom)
		imageInfo := benchmark.Image
		if driverType == driver.Runc || driverType == driver.Ctr {
			// legacy ctr mode and runc drivers need an exploded rootfs
			// first, verify thta a rootfs was provided in the benchmark YAML
			if benchmark.RootFs == "" {
				return benchResult{}, fmt.Errorf("No rootfs defined in the benchmark YAML; driver %s requires a root FS path", driverConfig.Type)
			}
			imageInfo = benchmark.RootFs
		}
		err := bench.Init(benchmark.Name, driverType, driverConfig.Binary, imageInfo, trace)
		if err != nil {
			return benchResult{}, err
		}
		benchInfo = bench.Info()
		if err = bench.Validate(); err != nil {
			return benchResult{}, fmt.Errorf("Error during bench validate: %v", err)
		}
		err = bench.Run(i, driverConfig.Iterations, benchmark.Commands)
		if err != nil {
			return benchResult{}, fmt.Errorf("Error during bench run: %v", err)
		}
		duration := bench.Elapsed()
		rate := float64(i*driverConfig.Iterations) / duration.Seconds()
		rates = append(rates, rate)
		log.Infof("%s: threads %d, iterations %d, rate: %6.2f", benchInfo, i, driverConfig.Iterations, rate)
	}
	result := benchResult{
		name:        benchInfo,
		threads:     driverConfig.Threads,
		iterations:  driverConfig.Iterations,
		threadRates: rates,
	}
	return result, nil
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

func readYaml(filename string) (benches.Benchmark, error) {
	var benchmarkYaml benches.Benchmark
	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return benchmarkYaml, fmt.Errorf("Can't read YAML file %q: %v", filename, err)
	}
	err = yaml.Unmarshal(yamlFile, &benchmarkYaml)
	if err != nil {
		return benchmarkYaml, fmt.Errorf("Can't unmarshal YAML file %q: %v", filename, err)
	}
	return benchmarkYaml, nil
}

func init() {
	RootCmd.AddCommand(runCmd)
	runCmd.PersistentFlags().StringVarP(&yamlFile, "benchmark", "b", "", "YAML file with benchmark definition")
	runCmd.PersistentFlags().BoolVarP(&trace, "trace", "t", false, "Enable per-container tracing during benchmark runs")
	runCmd.PersistentFlags().BoolVarP(&skipLimit, "skip-limit", "s", false, "Skip 'limit' benchmark run")
}
