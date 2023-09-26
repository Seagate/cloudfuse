//go:build windows

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.

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
	"github.com/Seagate/cloudfuse/cmd"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal/winservice"

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
		handler := &winservice.Cloudfuse{}
		run := svc.Run
		err = run(cmd.SvcName, handler)
		if err != nil {
			log.Err("Unable to start Windows service: %v", err.Error())
		}
	} else {
		_ = cmd.Execute()
		defer func() {
			if panicErr := recover(); panicErr != nil {
				log.Err("PANIC: %v", panicErr)
				panic(panicErr)
			}
		}()
	}
}
