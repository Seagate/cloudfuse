//go:build windows

package main

import (
	"fmt"
	"lyvecloudfuse/cmd"
	_ "lyvecloudfuse/common/log"
	"lyvecloudfuse/internal/windows_service"

	"golang.org/x/sys/windows/svc"
)

//go:generate ./cmd/componentGenerator.sh $NAME
//  To use go:generate run command   "NAME="component" go generate"

func main() {
	isInteractive, err := svc.IsWindowsService()
	if err != nil {
		fmt.Println(err)
	}

	if !isInteractive {
		_ = cmd.Execute()
	} else {
		handler := &windows_service.LyveCloudFuse{}
		svc.Run(windows_service.SvcName, handler)
	}
}
