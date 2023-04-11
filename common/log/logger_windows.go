//go:build windows

package log

import (
	"errors"
	"lyvecloudfuse/common"
	"strings"
)

// newLogger : Method to create Logger object
// On windows base is the default logger
func NewLogger(name string, config common.LogConfig) (Logger, error) {
	timeTracker = config.TimeTracker

	if len(strings.TrimSpace(config.Tag)) == 0 {
		config.Tag = common.FileSystemName
	}

	if name == "syslog" {
		sysLogger, err := newSysLogger(config.Level, config.Tag)
		if err != nil {
			if err == NoSyslogService {
				return NewLogger("base", config)
			}
		}
		return sysLogger, nil
	} else if name == "silent" {
		silentLogger := &SilentLogger{}
		return silentLogger, nil
	} else if name == "" || name == "default" || name == "base" || name == "syslog" {
		// TODO: make syslog default
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
	}
	return nil, errors.New("invalid logger type")
}
