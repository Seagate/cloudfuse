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
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/log"

	"github.com/spf13/cobra"
)

var unmountCmd = &cobra.Command{
	Use:               "unmount <mount path>",
	Short:             "Unmount container",
	Long:              "Unmount container",
	SuggestFor:        []string{"unmount", "unmnt"},
	Args:              cobra.ExactArgs(1),
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {
		mountPath := common.ExpandPath(args[0])

		disableRemountSystem, _ := cmd.Flags().GetBool("disable-remount-system")
		if runtime.GOOS == "windows" {
			disableRemountUser, _ := cmd.Flags().GetBool("disable-remount-user")
			mountPath = strings.ReplaceAll(common.ExpandPath(args[0]), "\\", "/")
			return unmountCloudfuseWindows(mountPath, disableRemountUser, disableRemountSystem)
		}

		if runtime.GOOS == "linux" && disableRemountSystem {
			err := uninstallService(mountPath)
			if err != nil {
				return fmt.Errorf(
					"failed to unmount and disable remount on restart for mount %s [%s]",
					mountPath,
					err.Error(),
				)
			}
		}

		lazy, _ := cmd.Flags().GetBool("lazy")
		if strings.Contains(args[0], "*") {
			mntPathPrefix := args[0]

			lstMnt, _ := common.ListMountPoints()
			for _, mntPath := range lstMnt {
				match, _ := regexp.MatchString(mntPathPrefix, mntPath)
				if match {
					err := unmountCloudfuse(mntPath, lazy, false)
					if err != nil {
						return fmt.Errorf("failed to unmount %s [%s]", mntPath, err.Error())
					}
				}
			}
		} else {
			err := unmountCloudfuse(args[0], lazy, false)
			if err != nil {
				return err
			}
		}
		return nil
	},
	ValidArgsFunction: func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if toComplete == "" {
			mntPts, _ := common.ListMountPoints()
			return mntPts, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveDefault
	},
}

// Attempts to unmount the directory and returns true if the operation succeeded
func unmountCloudfuse(mntPath string, lazy bool, silent bool) error {
	unmountCmd := []string{"fusermount3", "fusermount"}

	var errb bytes.Buffer
	var err error
	for _, umntCmd := range unmountCmd {
		var args []string
		if lazy {
			args = append(args, "-z")
		}
		args = append(args, "-u", mntPath)
		cliOut := exec.Command(umntCmd, args...)
		cliOut.Stderr = &errb
		_, err = cliOut.Output()

		if err == nil {
			log.Info("unmountBlobfuse2 : successfully unmounted %s", mntPath)
			if !silent {
				fmt.Println("Successfully unmounted", mntPath)
			}
			return nil
		}

		if !strings.Contains(err.Error(), "executable file not found") {
			fmt.Printf(
				"unmountCloudfuse : failed to unmount (%s : %s)\n",
				err.Error(),
				errb.String(),
			)
			break
		}
	}

	return fmt.Errorf("%s", errb.String()+" "+err.Error())
}

func init() {
	rootCmd.AddCommand(unmountCmd)
	unmountCmd.AddCommand(umntAllCmd)
	if runtime.GOOS != "windows" {
		unmountCmd.PersistentFlags().BoolP("lazy", "z", false, "Use lazy unmount")
	}

	if runtime.GOOS == "windows" {
		unmountCmd.Flags().
			Bool("disable-remount-user", false, "Disable remounting this mount on server restart as user.")
	}

	unmountCmd.Flags().
		Bool("disable-remount-system", false, "Disable remounting this mount on server restart as system.")
}
