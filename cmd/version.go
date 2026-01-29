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

	"github.com/Seagate/cloudfuse/common"

	"github.com/spf13/cobra"
)

var check bool

var versionCmd = &cobra.Command{
	Use:               "version",
	Short:             "Print the current version and optionally check for latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("cloudfuse version:", common.CloudfuseVersion)
		fmt.Println("git commit:", common.GitCommit)
		fmt.Println("commit date:", common.CommitDate)
		fmt.Println("go version:", common.GoVersion)
		fmt.Println("OS/Arch:", common.OsArch)
		if check {
			return VersionCheck()
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	versionCmd.Flags().
		BoolVar(&check, "check", false, "To check whether latest version exists or not")
}
