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
	"errors"
	"fmt"
	"runtime"

	"github.com/Seagate/cloudfuse/common"

	"github.com/spf13/cobra"
)

var umntAllCmd = &cobra.Command{
	Use:               "all",
	Short:             "Unmount all instances of Cloudfuse",
	Long:              "Unmount all instances of Cloudfuse",
	SuggestFor:        []string{"al", "all"},
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, _ []string) error {
		lstMnt, err := common.ListMountPoints()
		if err != nil {
			return fmt.Errorf("failed to list mount points [%s]", err.Error())
		}

		lazy, _ := cmd.Flags().GetBool("lazy")
		mountfound := 0
		unmounted := 0
		errMsg := "failed to unmount - \n"

		for _, mntPath := range lstMnt {
			mountfound += 1
			var err error
			if runtime.GOOS == "windows" {
				disableRemountUser, _ := cmd.Flags().GetBool("disable-remount-user")
				disableRemountSystem, _ := cmd.Flags().GetBool("disable-remount-system")
				err = unmountCloudfuseWindows(mntPath, disableRemountUser, disableRemountSystem)
			} else {
				err = unmountCloudfuse(mntPath, lazy)
			}
			if err == nil {
				unmounted += 1
			} else {
				errMsg += " " + mntPath + " - [" + err.Error() + "]\n"
			}
		}

		if mountfound == 0 {
			fmt.Println("Nothing to unmount")
		} else {
			fmt.Printf("%d of %d mounts were successfully unmounted\n", unmounted, mountfound)
		}

		if unmounted < mountfound {
			return errors.New(errMsg)
		}

		return nil
	},
}

func init() {
	if runtime.GOOS == "windows" {
		umntAllCmd.Flags().
			Bool("disable-remount-user", false, "Disable remounting this mount on server restart as user.")
		umntAllCmd.Flags().
			Bool("disable-remount-system", false, "Disable remounting this mount on server restart as system.")
	}
}
