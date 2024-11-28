package logging

// This wouldn't be needed if golang would let you override and overload functions.
// Maybe one day we can substantially reduce the logic here.

import (
	"bytes"
	"context"
	log "github.com/sirupsen/logrus"
	"runtime"
	"time"
)

type Entry struct {
	Loggers *Log
	Data    log.Fields
	Time    time.Time
	Level   log.Level
	Caller  *runtime.Frame
	Message string
	Buffer  *bytes.Buffer
	Context context.Context
	err     error
}

func (entry *Entry) FromLogrus(logrus *log.Entry, loggers Log) {
	entry.Loggers = &loggers
	entry.Data = logrus.Data
	entry.Time = logrus.Time
	entry.Level = logrus.Level
	entry.Caller = logrus.Caller
	entry.Message = logrus.Message
	entry.Buffer = logrus.Buffer
	entry.Context = logrus.Context
}

func (entry *Entry) Fatal(args ...interface{}) {
	for _, logger := range entry.Loggers.loggers {
		if len(args) == 1 {
			logger.WithFields(entry.Data).Warn(args[0])
		} else {
			logger.WithFields(entry.Data).Warn(args)
		}
	}
	entry.Loggers.LastResort.Fatal(args)
}

func (entry *Entry) Fatalf(format string, args ...interface{}) {
	for _, logger := range entry.Loggers.loggers {
		logger.WithFields(entry.Data).Warnf(format, args)
	}
	entry.Loggers.LastResort.Fatalf(format, args)
}

func (entry *Entry) Info(args ...interface{}) {
	for _, logger := range entry.Loggers.loggers {
		if len(args) == 1 {
			logger.WithFields(entry.Data).Info(args[0])
		} else {
			logger.WithFields(entry.Data).Info(args)
		}
	}
}

func (entry *Entry) Infof(format string, args ...interface{}) {
	for _, logger := range entry.Loggers.loggers {
		logger.WithFields(entry.Data).Infof(format, args)
	}
}

func (entry *Entry) Warn(args ...interface{}) {
	for _, logger := range entry.Loggers.loggers {
		if len(args) == 1 {
			logger.WithFields(entry.Data).Warn(args[0])
		} else {
			logger.WithFields(entry.Data).Warn(args)
		}
	}
}

func (entry *Entry) Warnf(format string, args ...interface{}) {
	for _, logger := range entry.Loggers.loggers {
		logger.WithFields(entry.Data).Warnf(format, args)
	}
}

func (entry *Entry) Debug(args ...interface{}) {
	for _, logger := range entry.Loggers.loggers {
		if len(args) == 1 {
			logger.WithFields(entry.Data).Debug(args[0])
		} else {
			logger.WithFields(entry.Data).Debug(args)
		}
	}
}

func (entry *Entry) Debugf(format string, args ...interface{}) {
	for _, logger := range entry.Loggers.loggers {
		logger.WithFields(entry.Data).Debugf(format, args)
	}
}

func (entry *Entry) Error(args ...interface{}) {
	for _, logger := range entry.Loggers.loggers {
		if len(args) == 1 {
			logger.WithFields(entry.Data).Error(args[0])
		} else {
			logger.WithFields(entry.Data).Error(args)
		}
	}
}

func (entry *Entry) Errorf(format string, args ...interface{}) {
	for _, logger := range entry.Loggers.loggers {
		logger.WithFields(entry.Data).Errorf(format, args)
	}
}
