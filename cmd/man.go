/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2024 Seagate Technology LLC and/or its Affiliates
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
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var manCmdInput = struct {
	outputLocation string
}{}

// manCmd represents the doc command
var manCmd = &cobra.Command{
	Use:    "man",
	Hidden: true,
	Short:  "Generates man page for Cloudfuse",
	Long:   "Generates Unix man pages for all cloudfuse commands.\nOutputs one man page file per command to the specified location.",
	Args:   cobra.NoArgs,
	Example: `  # Generate man pages to default location
  cloudfuse man

  # Generate man pages to custom directory
  cloudfuse man --output-location=/usr/local/share/man/man1`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// verify the output location
		f, err := os.Stat(manCmdInput.outputLocation)
		if err != nil && os.IsNotExist(err) {
			// create the output location if it does not exist yet
			if err = os.MkdirAll(manCmdInput.outputLocation, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create output location: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("cannot access output location: %w", err)
		} else if !f.IsDir() {
			return fmt.Errorf("output location is invalid as it is pointing to a file")
		}

		fixedDate := time.Unix(0, 0)

		header := &doc.GenManHeader{
			Title:   "cloudfuse",
			Section: "1",
			Date:    &fixedDate,
		}

		// dump the entire command tree's man pages into the folder
		err = doc.GenManTree(rootCmd, header, manCmdInput.outputLocation)
		if err != nil {
			return fmt.Errorf(
				"cannot generate man pages: %w",
				err,
			)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(manCmd)
	manCmd.PersistentFlags().StringVar(&manCmdInput.outputLocation, "output-location", "./doc",
		"where to put the generated man files")
	_ = manCmd.MarkPersistentFlagDirname("output-location")
}
