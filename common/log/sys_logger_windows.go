//go:build windows

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

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
	"fmt"
	"log"
	"lyvecloudfuse/common"

	"golang.org/x/sys/windows/svc/eventlog"
)

type SysLogger struct {
	level  common.LogLevel
	tag    string
	logger *log.Logger
}

var NoSyslogService = errors.New("failed to create syslog object")

func newSysLogger(lvl common.LogLevel, tag string) (*SysLogger, error) {
	l := &SysLogger{
		level: lvl,
		tag:   tag,
	}

	err := l.init() //sets up events..
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (l *SysLogger) GetLoggerObj() *log.Logger {
	return l.logger
}

func (l *SysLogger) SetLogLevel(level common.LogLevel) {
	l.level = level
	l.write(common.ELogLevel.LOG_CRIT().String(), "Log level reset to : %s", level.String())
}

func (l *SysLogger) GetType() string {
	return "log"
}

func (l *SysLogger) GetLogLevel() common.LogLevel {
	return l.level
}

func setupEvents() error {

	//TODO: set up / separate the InstallAsEventCreate() to only run from the installer.
	err := eventlog.InstallAsEventCreate("LyveCloudFuse", eventlog.Info|eventlog.Warning|eventlog.Error)

	if err.Error() == "Access is denied." {
		//this setup has already ran previously
		return nil
	}
	return err
}

func (l *SysLogger) init() error {

	err := setupEvents() //set up windows event registry for app
	if err != nil {
		return NoSyslogService
	}
	return nil
}

func newEvent(l *SysLogger, lvl string, msg string) error {

	wlog, err := eventlog.Open("LyveCloudFuse")
	if l.level == common.ELogLevel.LOG_DEBUG() {
		wlog.Info(uint32(101), msg)
	}
	if l.level == common.ELogLevel.LOG_TRACE() {
		wlog.Info(uint32(102), msg)
	}
	if l.level == common.ELogLevel.LOG_INFO() {
		wlog.Info(uint32(100), msg)
	}
	if l.level == common.ELogLevel.LOG_WARNING() {
		wlog.Warning(uint32(300), msg)
	}
	if l.level == common.ELogLevel.LOG_ERR() {
		wlog.Error(uint32(400), msg)
	}
	if l.level == common.ELogLevel.LOG_CRIT() {
		wlog.Error(uint32(401), msg)
	}
	return err
}

func (l *SysLogger) write(lvl string, format string, args ...interface{}) {

	msg := fmt.Sprintf(format, args...)
	//send this to be provided in the windows event.
	newEvent(l, lvl, msg)

}

func (l *SysLogger) Debug(format string, args ...interface{}) {
	if l.level == common.ELogLevel.LOG_DEBUG() {
		l.write(common.ELogLevel.LOG_DEBUG().String(), format, args...)
	}
}

func (l *SysLogger) Trace(format string, args ...interface{}) {
	if l.level == common.ELogLevel.LOG_TRACE() {
		l.write(common.ELogLevel.LOG_TRACE().String(), format, args...)
	}
}

func (l *SysLogger) Info(format string, args ...interface{}) {
	if l.level == common.ELogLevel.LOG_INFO() {
		l.write(common.ELogLevel.LOG_INFO().String(), format, args...)
	}
}

func (l *SysLogger) Warn(format string, args ...interface{}) {
	if l.level == common.ELogLevel.LOG_WARNING() {
		l.write(common.ELogLevel.LOG_WARNING().String(), format, args...)
	}
}

func (l *SysLogger) Err(format string, args ...interface{}) {
	if l.level == common.ELogLevel.LOG_ERR() {
		l.write(common.ELogLevel.LOG_ERR().String(), format, args...)
	}
}

func (l *SysLogger) Crit(format string, args ...interface{}) {
	if l.level == common.ELogLevel.LOG_CRIT() {
		l.write(common.ELogLevel.LOG_CRIT().String(), format, args...)
	}
}

// Methods not needed for syslog based logging
func (l *SysLogger) SetLogFile(name string) error {
	return nil
}

func (l *SysLogger) SetMaxLogSize(size int) {
}

func (l *SysLogger) SetLogFileCount(count int) {
}

func (l *SysLogger) Destroy() error {
	return nil
}

func (l *SysLogger) LogRotate() error {
	return nil
}
