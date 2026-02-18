//go:build windows

/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2026 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.

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

	"github.com/Seagate/cloudfuse/common"

	"golang.org/x/sys/windows/svc/eventlog"
)

type SysLogger struct {
	level          common.LogLevel
	tag            string
	logGoroutineID bool
	logger         *log.Logger
}

var NoSyslogService = errors.New("failed to create syslog object")

func newSysLogger(lvl common.LogLevel, tag string, logGoroutineID bool) (*SysLogger, error) {
	sysLog := &SysLogger{
		level:          lvl,
		tag:            tag,
		logGoroutineID: logGoroutineID,
	}

	err := sysLog.init() //sets up events..
	if err != nil {
		return nil, err
	}
	return sysLog, nil
}

// notice: logger object does not get populated since this is a relic from linux that has no windows equivalent.
func (sl *SysLogger) GetLoggerObj() *log.Logger {
	return sl.logger
}

func (sl *SysLogger) SetLogLevel(level common.LogLevel) {
	sl.level = level
	_ = sl.logEvent(common.ELogLevel.LOG_CRIT(), "Log level reset to :"+level.String())
}

func (sl *SysLogger) GetType() string {
	return "syslog"
}

func (sl *SysLogger) GetLogLevel() common.LogLevel {
	return sl.level
}

func (sl *SysLogger) init() error {

	//install or registry add should already have been ran.
	err := sl.logEvent(common.ELogLevel.LOG_DEBUG(), "starting event logger")
	if err != nil {
		return NoSyslogService
	}
	return nil
}

// populates an event in the windows event viewer

func (sl *SysLogger) logEvent(lvl common.LogLevel, msg string) error {

	wlog, err := eventlog.Open("Cloudfuse")
	if err != nil {
		return err
	}

	//the first argument of wlog.Info() is the event ID following the http convention
	//https://developer.mozilla.org/en-US/docs/Web/HTTP/Status
	switch lvl {
	case common.ELogLevel.LOG_DEBUG():
		err = wlog.Info(uint32(101), msg)
	case common.ELogLevel.LOG_TRACE():
		err = wlog.Info(uint32(102), msg)
	case common.ELogLevel.LOG_INFO():
		err = wlog.Info(uint32(100), msg)
	case common.ELogLevel.LOG_WARNING():
		err = wlog.Warning(uint32(300), msg)
	case common.ELogLevel.LOG_ERR():
		err = wlog.Error(uint32(400), msg)
	case common.ELogLevel.LOG_CRIT():
		err = wlog.Error(uint32(401), msg)
	}
	return err
}

func (sl *SysLogger) Debug(format string, args ...any) {
	if sl.level >= common.ELogLevel.LOG_DEBUG() {
		msg := fmt.Sprintf(format, args...)
		_ = sl.logEvent(common.ELogLevel.LOG_DEBUG(), msg)
	}
}

func (sl *SysLogger) Trace(format string, args ...any) {
	if sl.level >= common.ELogLevel.LOG_TRACE() {
		msg := fmt.Sprintf(format, args...)
		_ = sl.logEvent(common.ELogLevel.LOG_TRACE(), msg)
	}
}

func (sl *SysLogger) Info(format string, args ...any) {
	if sl.level >= common.ELogLevel.LOG_INFO() {
		msg := fmt.Sprintf(format, args...)
		_ = sl.logEvent(common.ELogLevel.LOG_INFO(), msg)
	}
}

func (sl *SysLogger) Warn(format string, args ...any) {
	if sl.level >= common.ELogLevel.LOG_WARNING() {
		msg := fmt.Sprintf(format, args...)
		_ = sl.logEvent(common.ELogLevel.LOG_WARNING(), msg)
	}
}

func (sl *SysLogger) Err(format string, args ...any) {
	if sl.level >= common.ELogLevel.LOG_ERR() {
		msg := fmt.Sprintf(format, args...)
		_ = sl.logEvent(common.ELogLevel.LOG_ERR(), msg)
	}
}

func (sl *SysLogger) Crit(format string, args ...any) {
	if sl.level >= common.ELogLevel.LOG_CRIT() {
		msg := fmt.Sprintf(format, args...)
		_ = sl.logEvent(common.ELogLevel.LOG_CRIT(), msg)
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
