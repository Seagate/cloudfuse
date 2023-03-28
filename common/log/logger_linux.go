//go:build linux

package log

import (
	"errors"
	"lyvecloudfuse/common"
	"strings"
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
