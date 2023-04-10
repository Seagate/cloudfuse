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

func (sl *SysLogger) GetLoggerObj() *log.Logger {
	return sl.logger
}

func (sl *SysLogger) SetLogLevel(level common.LogLevel) {
	sl.level = level
	sl.write(common.ELogLevel.LOG_CRIT().String(), "Log level reset to : %s", level.String())
}

func (sl *SysLogger) GetType() string {
	return "syslog"
}

func (sl *SysLogger) GetLogLevel() common.LogLevel {
	return sl.level
}

func (sl *SysLogger) init() error {

	//install or registry add should already have been ran.
	err := sl.logEvent(common.ELogLevel.LOG_DEBUG().String(), "first debug event test")
	if err != nil {
		return NoSyslogService
	}
	return nil
}

// populates an event in the windows event viewer

func (sl *SysLogger) logEvent(lvl string, msg string) error {

	wlog, err := eventlog.Open("LyveCloudFuse")

	if lvl == common.ELogLevel.LOG_DEBUG().String() {
		wlog.Info(uint32(101), msg)
		if err != nil {
			return err
		} else {
			println("cool, it worked without install")
		}
	}
	//the first argument of wlog.Info() is the event ID following the http convention
	//https://developer.mozilla.org/en-US/docs/Web/HTTP/Status

	switch level := sl.level; level {
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

func (sl *SysLogger) write(lvl string, format string, args ...interface{}) {

	msg := fmt.Sprintf(format, args...)
	//send this to be provided in the windows event.
	sl.logEvent(lvl, msg)

}

func (sl *SysLogger) Debug(format string, args ...interface{}) {
	if sl.level >= common.ELogLevel.LOG_DEBUG() {
		sl.write(common.ELogLevel.LOG_DEBUG().String(), format, args...)
	}
}

func (sl *SysLogger) Trace(format string, args ...interface{}) {
	if sl.level >= common.ELogLevel.LOG_TRACE() {
		sl.write(common.ELogLevel.LOG_TRACE().String(), format, args...)
	}
}

func (sl *SysLogger) Info(format string, args ...interface{}) {
	if sl.level >= common.ELogLevel.LOG_INFO() {
		sl.write(common.ELogLevel.LOG_INFO().String(), format, args...)
	}
}

func (sl *SysLogger) Warn(format string, args ...interface{}) {
	if sl.level >= common.ELogLevel.LOG_WARNING() {
		sl.write(common.ELogLevel.LOG_WARNING().String(), format, args...)
	}
}

func (sl *SysLogger) Err(format string, args ...interface{}) {
	if sl.level >= common.ELogLevel.LOG_ERR() {
		sl.write(common.ELogLevel.LOG_ERR().String(), format, args...)
	}
}

func (sl *SysLogger) Crit(format string, args ...interface{}) {
	if sl.level >= common.ELogLevel.LOG_CRIT() {
		sl.write(common.ELogLevel.LOG_CRIT().String(), format, args...)
	}
}

// Methods not needed for syslog based logging
func (sl *SysLogger) SetLogFile(name string) error {
	return nil
}

func (sl *SysLogger) SetMaxLogSize(size int) {
}

func (sl *SysLogger) SetLogFileCount(count int) {
}

func (sl *SysLogger) Destroy() error {
	return nil
}

func (sl *SysLogger) LogRotate() error {
	return nil
}
