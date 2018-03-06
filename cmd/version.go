// Copyright © 2018 Phil Estes <estesp@gmail.com>
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

	"github.com/spf13/cobra"
)

// filled in at compile time
var gitCommit = ""

const version = "0.4.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display bucketbench version and git commit information.",
	Long: `Display the bucketbench version and git commit information embedded in
the binary at build time.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("bucketbench v%s (commit: %s)\n", version, gitCommit)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
