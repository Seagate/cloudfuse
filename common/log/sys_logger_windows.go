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
	sysLog := &SysLogger{
		level: lvl,
		tag:   tag,
	}

	err := sysLog.init() //sets up events..
	if err != nil {
		return nil, err
	}
	return sysLog, nil
}

func (sysLog *SysLogger) GetLoggerObj() *log.Logger {
	return sysLog.logger
}

func (sysLog *SysLogger) SetLogLevel(level common.LogLevel) {
	sysLog.level = level
	sysLog.write(common.ELogLevel.LOG_CRIT().String(), "Log level reset to : %s", level.String())
}

func (sysLog *SysLogger) GetType() string {
	return "log"
}

func (sysLog *SysLogger) GetLogLevel() common.LogLevel {
	return sysLog.level
}

// sets up the windows registry for application to be able to report events into the event viewer
func setupEvents() error {

	//TODO: set up / separate the InstallAsEventCreate() to only run from the installer.
	err := eventlog.InstallAsEventCreate("LyveCloudFuse", eventlog.Info|eventlog.Warning|eventlog.Error)

	if err.Error() == "Access is denied." {
		//this setup has already ran previously
		return nil
	}
	return err
}

func (sysLog *SysLogger) init() error {

	err := setupEvents() //set up windows event registry for app
	if err != nil {
		return NoSyslogService
	}
	return nil
}

// populates an event in the windows event viewer

func (sysLog *SysLogger) logEvent(lvl string, msg string) error {

	//the first argument of wlog.Info() is the event ID following the http convention
	//https://developer.mozilla.org/en-US/docs/Web/HTTP/Status
	wlog, err := eventlog.Open("LyveCloudFuse")
	switch level := sysLog.level; level {
	case common.ELogLevel.LOG_DEBUG():
		wlog.Info(uint32(101), msg)
	case common.ELogLevel.LOG_TRACE():
		wlog.Info(uint32(102), msg)
	case common.ELogLevel.LOG_INFO():
		wlog.Info(uint32(100), msg)
	case common.ELogLevel.LOG_WARNING():
		wlog.Info(uint32(300), msg)
	case common.ELogLevel.LOG_ERR():
		wlog.Info(uint32(400), msg)
	case common.ELogLevel.LOG_CRIT():
		wlog.Info(uint32(401), msg)
	}
	return err
}

func (sysLog *SysLogger) write(lvl string, format string, args ...interface{}) {

	msg := fmt.Sprintf(format, args...)
	//send this to be provided in the windows event.
	sysLog.logEvent(lvl, msg)

}

func (sysLog *SysLogger) Debug(format string, args ...interface{}) {
	if sysLog.level >= common.ELogLevel.LOG_DEBUG() {
		sysLog.write(common.ELogLevel.LOG_DEBUG().String(), format, args...)
	}
}

func (sysLog *SysLogger) Trace(format string, args ...interface{}) {
	if sysLog.level >= common.ELogLevel.LOG_TRACE() {
		sysLog.write(common.ELogLevel.LOG_TRACE().String(), format, args...)
	}
}

func (sysLog *SysLogger) Info(format string, args ...interface{}) {
	if sysLog.level >= common.ELogLevel.LOG_INFO() {
		sysLog.write(common.ELogLevel.LOG_INFO().String(), format, args...)
	}
}

func (sysLog *SysLogger) Warn(format string, args ...interface{}) {
	if sysLog.level >= common.ELogLevel.LOG_WARNING() {
		sysLog.write(common.ELogLevel.LOG_WARNING().String(), format, args...)
	}
}

func (sysLog *SysLogger) Err(format string, args ...interface{}) {
	if sysLog.level >= common.ELogLevel.LOG_ERR() {
		sysLog.write(common.ELogLevel.LOG_ERR().String(), format, args...)
	}
}

func (sysLog *SysLogger) Crit(format string, args ...interface{}) {
	if sysLog.level >= common.ELogLevel.LOG_CRIT() {
		sysLog.write(common.ELogLevel.LOG_CRIT().String(), format, args...)
	}
}

// Methods not needed for syslog based logging
func (sysLog *SysLogger) SetLogFile(name string) error {
	return nil
}

func (sysLog *SysLogger) SetMaxLogSize(size int) {
}

func (sysLog *SysLogger) SetLogFileCount(count int) {
}

func (sysLog *SysLogger) Destroy() error {
	return nil
}

func (sysLog *SysLogger) LogRotate() error {
	return nil
}
