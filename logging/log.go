package logging

// This wouldn't be needed if golang would let you override and overload functions.
// Maybe one day we can substantially reduce the logic here.

import (
	"errors"
	log "github.com/sirupsen/logrus"
	logrus_syslog "github.com/sirupsen/logrus/hooks/syslog"
	"github.com/spf13/viper"
	"io"
	"log/syslog"
	"os"
	"strings"
)

type Log struct {
	loggers    []*log.Logger
	config     *viper.Viper
	lastResort log.Logger
}

func (logger *Log) FromConfig(conf *viper.Viper) {
	logger.lastResort = log.Logger{Out: os.Stdout}
	logger.config = conf
	logKeys := conf.AllKeys()
	for _, key := range logKeys {
		k := strings.Split(key, ".")[0]
		config := conf.Sub(k)

		outConf := config.GetString("out")

		var hook *logrus_syslog.SyslogHook = nil
		var stdout bool
		var out io.Writer
		var logFile *os.File

		switch outConf {
		case "syslog":
			var err error
			proto := config.GetString("protocol")
			addr := config.GetString("addr")
			hook, err = logrus_syslog.NewSyslogHook(proto, addr, syslog.LOG_INFO, "")
			if err != nil {
				logger.lastResort.Warn("Error setting up syslog logger")
			}
		case "stdout":
			if stdout {
				logger.lastResort.Fatal("Multiple stdout loggers are not possible. Please correct your config!")
			} else {
				stdout = true
				out = os.Stdout
			}
		case "file":
			var err error
			path := config.GetString("path")
			if _, err = os.Stat(path); errors.Is(err, os.ErrNotExist) {
				logFile, err = os.OpenFile(path, os.O_CREATE, 0666)
				if err != nil {
					logger.lastResort.Errorf("Error opening log file %s: %s", path, err)
				}
				defer func(logFile *os.File) {
					err := logFile.Close()
					if err != nil {
						logger.lastResort.Warn("Error closing logfile")
					}
				}(logFile)
			} else {
				logFile, err = os.OpenFile(path, os.O_APPEND, 0666)
				if err != nil {
					logger.lastResort.Errorf("Error opening log file %s: %s", path, err)
				}
				defer func(logFile *os.File) {
					err := logFile.Close()
					if err != nil {
						logger.lastResort.Warn("Error closing logfile")
					}
				}(logFile)
			}
			out = logFile
		}

		l := log.Logger{
			Out: out,
		}

		if hook != nil {
			l.AddHook(hook)
		}

		switch config.GetString("formatter") {
		case "default":
			l.SetFormatter(&log.TextFormatter{})
		case "json":
			l.SetFormatter(&log.JSONFormatter{})
		}

		logger.loggers = append(logger.loggers, &l)
	}
}

func (logger Log) WithFields(fields log.Fields) *Entry {
	logrus := log.WithFields(fields)
	entry := Entry{}
	entry.FromLogrus(logrus, logger)

	return &entry
}

func (logger Log) Fatal(message string) {
	for _, logger := range logger.loggers {
		logger.Warn(message)
	}
	logger.lastResort.Fatal(message)
}

func (logger Log) Fatalf(format string, args ...interface{}) {
	for _, logger := range logger.loggers {
		logger.Warnf(format, args)
	}
	logger.lastResort.Fatalf(format, args)
}

func (logger Log) Info(message string) {
	for _, logger := range logger.loggers {
		logger.Info(message)
	}
}

func (logger Log) Infof(format string, args ...interface{}) {
	for _, logger := range logger.loggers {
		logger.Infof(format, args)
	}
}

func (logger Log) Warn(message string) {
	for _, logger := range logger.loggers {
		logger.Info(message)
	}
}

func (logger Log) Warnf(format string, args ...interface{}) {
	for _, logger := range logger.loggers {
		logger.Warnf(format, args)
	}
}

func (logger Log) Debug(message string) {
	for _, logger := range logger.loggers {
		logger.Debug(message)
	}
}

func (logger Log) Debugf(format string, args ...interface{}) {
	for _, logger := range logger.loggers {
		logger.Debugf(format, args)
	}
}

func (logger Log) Error(message string) {
	for _, logger := range logger.loggers {
		logger.Error(message)
	}
}

func (logger Log) Errorf(format string, args ...interface{}) {
	for _, logger := range logger.loggers {
		logger.Errorf(format, args)
	}
}
