package main

import (
	"golang.org/x/sys/windows/svc/eventlog"
)

// sets up the windows registry for application to be able to report events into the event viewer
func setupEvents() error {

	//TODO: set up / separate the InstallAsEventCreate() to only run from the installer.
	err := eventlog.InstallAsEventCreate("LyveCloudFuse", eventlog.Info|eventlog.Warning|eventlog.Error)

	return err
}

func main() {
	err := setupEvents()

	if err.Error() == "Access is denied." {
		//this error will typically take place upon not running with sufficient privileges.
		println("you should run this as admin and try again")
	} else if err.Error() == "registry already exists" {
		println("you already have this installed. You're all set")
	} else if err != nil {
		println("we ran into the following error from attempting to install:" + err.Error())
	}
}
