//go:build windows

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates

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

package main

import (
	"strings"

	"golang.org/x/sys/windows/svc/eventlog"
)

// sets up the windows registry for application to be able to report events into the event viewer
// you will need to run this as an administrator. if you are running this inside vscode, run vscode as adamin.
func setupEvents() error {

	//TODO: set up / separate the InstallAsEventCreate() to only run from the installer.
	err := eventlog.InstallAsEventCreate("Cloudfuse", eventlog.Info|eventlog.Warning|eventlog.Error)

	return err
}

func main() {
	err := setupEvents()

	if err.Error() == "Access is denied." {
		//this error will typically take place upon not running with sufficient privileges.
		println("you should run this as admin and try again")
	} else if strings.Contains(err.Error(), "registry key already exists") {
		println("you already have this installed. You're all set")
	} else if err != nil {
		println("we ran into the following error from attempting to install:" + err.Error())
	}
}
