//go:build linux

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2024 Seagate Technology LLC and/or its Affiliates
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

package log

import (
	"errors"
	"strings"

	"github.com/Seagate/cloudfuse/common"
)

// newLogger : Method to create Logger object
func NewLogger(name string, config common.LogConfig) (Logger, error) {
	timeTracker = config.TimeTracker

	if len(strings.TrimSpace(config.Tag)) == 0 {
		config.Tag = common.FileSystemName
	}

	if name == "base" {
		baseLogger, err := newBaseLogger(LogFileConfig{
			LogFile:      config.FilePath,
			LogLevel:     config.Level,
			LogSize:      config.MaxFileSize * 1024 * 1024,
			LogFileCount: int(config.FileCount),
			LogTag:       config.Tag,
		})
		if err != nil {
			return nil, err
		}
		return baseLogger, nil
	} else if name == "silent" {
		silentLogger := &SilentLogger{}
		return silentLogger, nil
	} else if name == "" || name == "default" || name == "syslog" {
		sysLogger, err := newSysLogger(config.Level, config.Tag)
		if err != nil {
			if err == NoSyslogService {
				// Syslog service does not exists on this system
				// fallback to file based logging.
				return NewLogger("base", config)
			}
			return nil, err
		}
		return sysLogger, nil
	}
	return nil, errors.New("invalid logger type")
}
