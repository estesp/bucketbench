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

	"os"
	"text/tabwriter"

	"github.com/estesp/bucketbench/benches"
	"github.com/estesp/bucketbench/driver"
	"github.com/go-yaml/yaml"
	"github.com/montanaflynn/stats"
	log "github.com/sirupsen/logrus"
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
	statistics  [][]benches.RunStatistics
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
		limit.Init("", driver.Null, "", "", "", trace)
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
		stats     [][]benches.RunStatistics
		benchInfo string
	)
	driverType := driver.StringToType(driverConfig.Type)
	stats = make([][]benches.RunStatistics, driverConfig.Threads)

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
		err := bench.Init(benchmark.Name, driverType, driverConfig.ClientPath, imageInfo, benchmark.Command, trace)
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
		stats[i-1] = bench.Stats()
		log.Infof("%s: threads %d, iterations %d, rate: %6.2f", benchInfo, i, driverConfig.Iterations, rate)
	}
	result := benchResult{
		name:        benchInfo,
		threads:     driverConfig.Threads,
		iterations:  driverConfig.Iterations,
		threadRates: rates,
		statistics:  stats,
	}
	return result, nil
}

func outputRunDetails(maxThreads int, results []benchResult) {
	w := tabwriter.NewWriter(os.Stdout, 10, 4, 2, ' ', tabwriter.AlignRight)

	fmt.Printf("\nSUMMARY TIMINGS/THREAD RATES\n\n")
	fmt.Fprintf(w, " \tIter/Thd\t1 thrd")
	for i := 2; i <= maxThreads; i++ {
		fmt.Fprintf(w, "\t%d thrds", i)
	}
	fmt.Fprintln(w, "\t ")

	for _, result := range results {
		fmt.Fprintf(w, "%s\t%d\t%7.2f", result.name, result.iterations, result.threadRates[0])
		for i := 1; i < result.threads; i++ {
			fmt.Fprintf(w, "\t%7.2f", result.threadRates[i])
		}
		fmt.Fprintln(w, "\t ")
	}
	w.Flush()
	fmt.Println("")

	cmdList := []string{"run", "pause", "resume", "stop", "delete"}
	fmt.Printf("DETAILED COMMAND TIMINGS/STATISTICS\n")
	// output per-command timings across the runs as well
	for _, result := range results {
		if result.name == "Limit" {
			// the limit "benchmark" has no detailed statistics
			continue
		}
		for i := 0; i < result.threads; i++ {
			fmt.Fprintf(w, "%s:%d\tMin\tMax\tAvg\tMedian\tStddev\tErrors\t\n", result.name, i+1)
			cmdTimings := parseStats(result.statistics[i])
			// given we are working with a map, but we want consistent ordering in the output
			// we walk a slice of commands in a natural/expected order and output stats for
			// those that were used during the specific run
			for _, cmd := range cmdList {
				if stats, ok := cmdTimings[cmd]; ok {
					fmt.Fprintf(w, "%s\t%6.2f\t%6.2f\t%6.2f\t%6.2f\t%6.2f\t%d\t\n", cmd, stats.min, stats.max, stats.avg, stats.median, stats.stddev, stats.errors)
				}
			}
		}
		fmt.Println("")
	}
	w.Flush()
}

type statResults struct {
	min    float64
	max    float64
	avg    float64
	median float64
	stddev float64
	errors int
}

func parseStats(statistics []benches.RunStatistics) map[string]statResults {
	result := make(map[string]statResults)
	durationSeq := make(map[string][]float64)
	errorSeq := make(map[string][]int)
	iterations := len(statistics)

	durationKeys := make([]string, len(statistics[0].Durations))
	i := 0
	for k := range statistics[0].Durations {
		durationKeys[i] = k
		i++
	}
	for i := 0; i < iterations; i++ {
		for key, duration := range statistics[i].Durations {
			durationSeq[key] = append(durationSeq[key], float64(duration))
		}
		for key, errors := range statistics[i].Errors {
			errorSeq[key] = append(errorSeq[key], errors)
		}
	}
	for _, key := range durationKeys {
		// take the durations for this key and perform
		// several math/statistical functions:
		min, err := stats.Min(durationSeq[key])
		if err != nil {
			log.Errorf("Error finding stats.Min(): %v", err)
		}
		max, err := stats.Max(durationSeq[key])
		if err != nil {
			log.Errorf("Error finding stats.Max(): %v", err)
		}
		average, err := stats.Mean(durationSeq[key])
		if err != nil {
			log.Errorf("Error finding stats.Average(): %v", err)
		}
		median, err := stats.Median(durationSeq[key])
		if err != nil {
			log.Errorf("Error finding stats.Median(): %v", err)
		}
		stddev, err := stats.StandardDeviation(durationSeq[key])
		if err != nil {
			log.Errorf("Error finding stats.StdDev(): %v", err)
		}
		var errors int
		if errorSlice, ok := errorSeq[key]; ok {
			errors = intSum(errorSlice)
		}
		result[key] = statResults{
			min:    min,
			max:    max,
			avg:    average,
			median: median,
			stddev: stddev,
			errors: errors,
		}
	}
	return result
}

func intSum(slice []int) int {
	var total int
	for _, val := range slice {
		total += val
	}
	return total
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
