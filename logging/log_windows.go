//go:build windows

package logging

// This wouldn't be needed if golang would let you override and overload functions.
// Maybe one day we can substantially reduce the logic here.

import (
	"errors"
	"github.com/knadh/koanf/v2"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"strings"
)

type Log struct {
	loggers    []*log.Logger
	config     *koanf.Koanf
	LastResort *log.Logger
	Localizer  *i18n.Localizer
}

func (logger *Log) FromConfig(conf *koanf.Koanf) {
	logger.config = conf
	if conf != nil {
		logKeys := conf.Keys()
		for _, key := range logKeys {
			k := strings.Split(key, ".")[0]
			config := conf.Cut(k)
			if strings.Contains(strings.Split(key, ".")[1], "out") {
				outConf := config.String("out")

				var stdout bool
				var out io.Writer
				var logFile *os.File

				switch outConf {
				case "stdout":
					if stdout {
						message := logger.Localizer.MustLocalize(&i18n.LocalizeConfig{
							MessageID: "LogstdoutError",
						})
						logger.LastResort.Fatal(message)
					} else {
						stdout = true
						out = os.Stdout
					}
				case "file":
					var err error
					path := config.String("path")
					if _, err = os.Stat(path); errors.Is(err, os.ErrNotExist) {
						logFile, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0666)
						if err != nil {
							message := logger.Localizer.MustLocalize(&i18n.LocalizeConfig{
								MessageID: "LogFileError",
								TemplateData: map[string]interface{}{
									"Error": err.Error(),
									"Path":  path,
								},
							})
							logger.LastResort.Error(message)
						}
					} else {
						logFile, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0666)
						if err != nil {
							message := logger.Localizer.MustLocalize(&i18n.LocalizeConfig{
								MessageID: "LogFileError",
								TemplateData: map[string]interface{}{
									"Error": err.Error(),
									"Path":  path,
								},
							})
							logger.LastResort.Error(message)
						}
					}
					out = logFile
				}

				l := log.New()

				l.SetOutput(out)

				switch config.String("formatter") {
				case "default":
					l.SetFormatter(&log.TextFormatter{})
				case "json":
					l.SetFormatter(&log.JSONFormatter{})
				}

				logger.loggers = append(logger.loggers, l)
			}
		}
	} else {
		message := logger.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "LogLastResortOnly",
		})
		logger.LastResort.Warn(message)
		logger.loggers = append(logger.loggers, logger.LastResort)
	}
}

func (logger Log) WithFields(fields log.Fields) *Entry {
	logrus := log.WithFields(fields)
	entry := Entry{}
	entry.FromLogrus(logrus, logger)

	return &entry
}

func (logger *Log) SetLevel(level log.Level) {
	for _, logInstance := range logger.loggers {
		logInstance.SetLevel(level)
	}
}

func (logger Log) Fatal(args ...interface{}) {
	for _, logInstance := range logger.loggers {
		if len(args) == 1 {
			logInstance.Warn(args[0])
		} else {
			logInstance.Warn(args)
		}
	}
	logger.LastResort.Fatal(args)
}

func (logger Log) Fatalf(format string, args ...interface{}) {
	for _, logInstance := range logger.loggers {
		logInstance.Warnf(format, args)
	}
	logger.LastResort.Fatalf(format, args)
}

func (logger Log) Info(args ...interface{}) {
	for _, logInstance := range logger.loggers {
		if len(args) == 1 {
			logInstance.Info(args[0])
		} else {
			logInstance.Info(args)
		}
	}
}

func (logger Log) Infof(format string, args ...interface{}) {
	for _, logInstance := range logger.loggers {
		logInstance.Infof(format, args)
	}
}

func (logger Log) Warn(args ...interface{}) {
	for _, logInstance := range logger.loggers {
		if len(args) == 1 {
			logInstance.Warn(args[0])
		} else {
			logInstance.Warn(args)
		}
	}
}

func (logger Log) Warnf(format string, args ...interface{}) {
	for _, logInstance := range logger.loggers {
		logInstance.Warnf(format, args)
	}
}

func (logger Log) Debug(args ...interface{}) {
	for _, logInstance := range logger.loggers {
		if len(args) == 1 {
			logInstance.Debug(args[0])
		} else {
			logInstance.Debug(args)
		}
	}
}

func (logger Log) Debugf(format string, args ...interface{}) {
	for _, logInstance := range logger.loggers {
		logInstance.Debugf(format, args)
	}
}

func (logger Log) Error(args ...interface{}) {
	for _, logInstance := range logger.loggers {
		if len(args) == 1 {
			logInstance.Error(args[0])
		} else {
			logInstance.Error(args)
		}
	}
}

func (logger Log) Errorf(format string, args ...interface{}) {
	for _, logInstance := range logger.loggers {
		logInstance.Errorf(format, args)
	}
}

func (logger Log) Panic(args ...interface{}) {
	for _, logInstance := range logger.loggers {
		if len(args) == 1 {
			logInstance.Warn(args[0])
		} else {
			logInstance.Warn(args)
		}
	}
	logger.LastResort.Panic(args)
}

func (logger Log) Panicf(format string, args ...interface{}) {
	for _, logInstance := range logger.loggers {
		logInstance.Warnf(format, args)
	}
	logger.LastResort.Panicf(format, args)
}

func (logger Log) Println(args ...interface{}) {
	for _, logInstance := range logger.loggers {
		if len(args) == 1 {
			logInstance.Println(args[0])
		} else {
			logInstance.Println(args)
		}
	}
}

func (logger Log) Printf(format string, args ...interface{}) {
	for _, logInstance := range logger.loggers {
		logInstance.Printf(format, args)
	}
}
