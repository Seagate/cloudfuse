/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:        "generate <component name>",
	Hidden:     true,
	Short:      "Generate a new component for Cloudfuse",
	Long:       "Generate a new cloudfuse component with boilerplate code.\nRuns the componentGenerator.sh script to scaffold the component structure.",
	SuggestFor: []string{"gen", "gener"},
	Args:       cobra.ExactArgs(1),
	Example:    "  cloudfuse generate mycomponent",
	RunE: func(cmd *cobra.Command, args []string) error {
		componentName := args[0]
		script := exec.Command("./cmd/componentGenerator.sh", componentName)
		script.Stdout = os.Stdout
		script.Stderr = os.Stdout

		if err := script.Run(); err != nil {
			return fmt.Errorf("failed to generate new component [%s]", err.Error())
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
}
