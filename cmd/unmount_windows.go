//go:build windows

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

	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal/winservice"
)

func unmountCloudfuseWindows(mountPath string, disableRemountUser bool, disableRemountSystem bool) error {
	// Remove the mount from json file so it does not remount on restart.
	if disableRemountUser || disableRemountSystem {
		err := winservice.RemoveMountJSON(mountPath, disableRemountSystem)
		// If error is not nill then ignore it
		if err != nil {
			log.Err("failed to remove entry from json file [%s]. Are you sure this mount was enabled for remount?", err.Error())
		}
	}

	// Check with winfsp to see if this is currently mounted
	ret, err := winservice.IsMounted(mountPath)
	if err != nil {
		return fmt.Errorf("failed to validate options [%s]", err.Error())
	} else if !ret {
		return fmt.Errorf("nothing is mounted here")
	}

	err = winservice.StopMount(mountPath)
	if err != nil {
		return fmt.Errorf("failed to unmount instance [%s]", err.Error())
	}

	return nil
}
