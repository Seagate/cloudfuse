//go:build windows

package main

import (
	"lyvecloudfuse/cmd"
	"lyvecloudfuse/common/log"
	"lyvecloudfuse/internal/winservice"

	"golang.org/x/sys/windows/svc"
)

//go:generate ./cmd/componentGenerator.sh $NAME
//  To use go:generate run command   "NAME="component" go generate"

func main() {
	isService, err := svc.IsWindowsService()
	if err != nil {
		log.Err("Unable to determine if running as Windows service: %v", err.Error())
	}

	if isService {
		handler := &winservice.LyveCloudFuse{}
		run := svc.Run
		err = run(cmd.SvcName, handler)
		if err != nil {
			log.Err("Unable to start Windows service: %v", err.Error())
		}
	} else {
		_ = cmd.Execute()
	}
}
