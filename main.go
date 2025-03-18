/*
Copyright © 2022 aetaric <aetaric@gmail.com>
*/
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/aetaric/checkrr/logging"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	"github.com/aetaric/checkrr/check"
	"github.com/aetaric/checkrr/features"
	"github.com/aetaric/checkrr/webserver"
	"github.com/common-nighthawk/go-figure"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	bolt "go.etcd.io/bbolt"
)

var scheduler *cron.Cron
var flagSet pflag.FlagSet

var k = koanf.New(".")

var cfgFile string
var checkVer bool
var oneShot bool
var debug bool

var web webserver.Webserver
var DB *bolt.DB
var logger *logging.Log

//go:embed locale/*.toml
var LocaleFS embed.FS
var bundle *i18n.Bundle
var localizer *i18n.Localizer

// These vars are set at compile time by goreleaser
var version = "development"
var commit string
var date string
var builtBy string

func main() {
	// Setup pre logging logger and logger of last resort
	logger = &logging.Log{LastResort: log.New()}

	// Prints the Banner
	ascii := figure.NewColorFigure("checkrr", "block", "green", true)
	ascii.Print()
	printVersion()

	// Sets up flags
	initFlags()

	// Reads in config file
	initConfig()

	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	err := fs.WalkDir(LocaleFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		} else {
			if !d.IsDir() {
				_, err = bundle.LoadMessageFileFS(LocaleFS, path)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return
	}
	if k.String("lang") == "" {
		// defaulting to english to prevent crashing
		localizer = i18n.NewLocalizer(bundle, "en")
	} else {
		localizer = i18n.NewLocalizer(bundle, k.String("lang"))
	}

	// Setup logger
	logger.Localizer = localizer
	logger.FromConfig(k.Cut("logs"), k.Bool("checkrr.debug"))

	// Verify ffprobe is in PATH
	_, binpatherr := exec.LookPath("ffprobe")
	if binpatherr != nil {
		message := localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "NoFFProbe",
		})
		logger.WithFields(log.Fields{"startup": true}).Fatal(message)
	}

	// debug
	if debug {
		logger.SetLevel(log.DebugLevel)
	}

	// Output Version if requested
	if checkVer {
		os.Exit(0)
	}

	// Setup SIGINT and SIGTERM handling
	term := make(chan os.Signal, 1)
	signal.Notify(term, syscall.SIGINT, syscall.SIGTERM)

	// Handle SIGHUP
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)

	// Channel to render time after execution
	rendertime := make(chan []string, 1)

	// Channel for killing the webserver if enabled
	//webstop := make(chan bool, 1)

	// Channel for sending data to webserver
	webdata := make(chan []string)

	// Close the channels on exit
	defer func() {
		signal.Stop(term)
		signal.Stop(hup)
	}()

	// Setup Database
	if k.String("checkrr.database") != "" {
		var err error

		DB, err = bolt.Open(k.String("checkrr.database"), 0600, nil)
		if err != nil {
			message := localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "DBSetupError",
				TemplateData: map[string]interface{}{
					"Error": err,
				},
			})
			logger.WithFields(log.Fields{"startup": true}).Fatal(message)
		}
		defer func(DB *bolt.DB) {
			err := DB.Close()
			if err != nil {
				message := localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "DBCloseError",
					TemplateData: map[string]interface{}{
						"Error": err,
					},
				})
				logger.WithFields(log.Fields{"shutdown": true}).Fatal(message)
			}
		}(DB)

		err = DB.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte("Checkrr"))
			if err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}
			return nil
		})
		if err != nil {
			message := localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "DBSetupError",
				TemplateData: map[string]interface{}{
					"Error": err,
				},
			})
			logger.WithFields(log.Fields{"startup": true, "database": "setup"}).Fatal(message)
		}

		err = DB.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte("Checkrr-files"))
			if err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}
			return nil
		})
		if err != nil {
			message := localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "DBSetupError",
				TemplateData: map[string]interface{}{
					"Error": err,
				},
			})
			logger.WithFields(log.Fields{"startup": true, "database": "setup"}).Fatal(message)
		}

		testRunning := false
		statsCleanup := features.Stats{}

		err = DB.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte("Checkrr-stats"))
			if err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}

			b := tx.Bucket([]byte("Checkrr-stats"))
			statdata := b.Get([]byte("current-stats"))
			if len(statdata) != 0 {
				err = json.Unmarshal(statdata, &statsCleanup)
				if err != nil {
					return err
				}

				if statsCleanup.Running {
					message := localizer.MustLocalize(&i18n.LocalizeConfig{
						MessageID: "DBCleanup",
					})
					logger.WithFields(log.Fields{"startup": true}).Warn(message)
					statsCleanup.Running = false
					testRunning = true
				}
			}

			return nil
		})
		if err != nil {
			message := localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "DBCleanupError",
				TemplateData: map[string]interface{}{
					"Error": err,
				},
			})
			logger.WithFields(log.Fields{"startup": true, "database": "setup"}).Fatal(message)
		}

		if testRunning {
			err := DB.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("Checkrr-stats"))
				jsonData, er := json.Marshal(statsCleanup)
				if er != nil {
					return er
				}
				err := b.Put([]byte("current-stats"), jsonData)
				return err
			})
			if err != nil {
				message := localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "DBCleanupError",
					TemplateData: map[string]interface{}{
						"Error": err.Error(),
					},
				})
				logger.WithFields(log.Fields{"Module": "Stats", "DB Update": "Failure"}).Warn(message)
			}
		}
	} else {
		message := localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "DBMissing",
		})
		logger.WithFields(log.Fields{"startup": true}).Fatal(message)
	}

	// Build checkrr from config
	c := check.Checkrr{Chan: &rendertime, DB: DB, Logger: logger, FullConfig: k, Localizer: localizer}
	c.FromConfig(k.Cut("checkrr"))

	// Webserver Init
	var runWeb bool = false
	if len(k.Cut("webserver").Keys()) > 0 {
		web = webserver.Webserver{DB: DB, FullConfig: k}
		web.FromConfig(k.Cut("webserver"), webdata, &c, localizer)
		runWeb = true
	}

	if oneShot {
		if runWeb {
			go web.Run()
		}
		c.Run()
	} else {
		// Setup Cron runner.
		var id cron.EntryID
		scheduler = cron.New()
		id, _ = scheduler.AddJob(k.String("checkrr.cron"), &c)
		web.AddScheduler(scheduler, id)
		if runWeb {
			go web.Run()
		}
		scheduler.Start()
		message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "ScheduleNextRun",
			TemplateData: map[string]interface{}{
				"Time": scheduler.Entry(id).Next.String(),
			},
		})
		logger.Info(message)

		// Blocks forever waiting on Signals from the system or user
		for {
			select {
			case <-term:
				// Shutdown process on SIGINT or SIGTERM
				if c.Stats.Running {
					c.Stats.Stop()
					c.Stats.Render()
				}
				scheduler.Stop()
				os.Exit(0)
			case <-rendertime:
				// Output next run time
				message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "ScheduleNextRun",
					TemplateData: map[string]interface{}{
						"Time": scheduler.Entry(id).Next.String(),
					},
				})
				logger.Info(message)
			case <-hup:
				// Reload config and reinit scheduler on SIGHUP
				initConfig()
				scheduler.Remove(id)
				scheduler.Stop()
				id, _ = scheduler.AddJob(k.String("checkrr.cron"), &c)
				scheduler.Start()
				message := c.Localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "ConfigReload",
				})
				logger.Info(message)
				message = c.Localizer.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "ScheduleNextRun",
					TemplateData: map[string]interface{}{
						"Time": scheduler.Entry(id).Next.String(),
					},
				})
				logger.Info(message)
			}
		}
	}
}

func printVersion() {
	fmt.Printf("Checkrr version %s\n Commit: %s\n Built On: %s\n Built By: %s\n", version, commit, date, builtBy)
}

func initFlags() {
	// Setup flags
	flagSet = *pflag.NewFlagSet("checkrr", pflag.ExitOnError)
	flagSet.BoolVarP(&checkVer, "version", "v", false, "Prints version info")
	flagSet.BoolVarP(&oneShot, "run-once", "o", false, "Runs Checkrr once and then exits; Default is running as a daemon")
	flagSet.BoolVarP(&debug, "debug", "d", false, "Enables debug logging")

	flagSet.StringVarP(&cfgFile, "config-file", "c", "", "Specify a config file to use")

	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		logger.LastResort.WithFields(log.Fields{"startup": true}).Warnf("unable to parse commandline flags: %s", err)
	}
}

func initConfig() {
	if cfgFile != "" {
		if err := k.Load(file.Provider(cfgFile), yaml.Parser()); err != nil {
			logger.LastResort.Fatalf("Error loading config file: %s\n %s", cfgFile, err)
		}
	} else {
		logger.LastResort.Warn("No Config file specified, trying to load a default...")
		// Find home directory.
		home, _ := os.UserHomeDir()

		paths := []string{"."}

		runtimeOS := runtime.GOOS
		switch runtimeOS {
		case "windows":
			paths = append(paths, "C:/")
		default:
			paths = append(paths, "/etc", "/etc/checkrr")
		}

		paths = append(paths, home)

		for _, path := range paths {
			err := k.Load(file.Provider(fmt.Sprintf("%s/checkrr.yaml", path)), yaml.Parser())
			if err != nil {
				logger.LastResort.Infof("Couldn't load config at: %s/checkrr.yaml", path)
			} else {
				logger.LastResort.Infof("Found config at: %s/checkrr.yaml", path)
				return
			}

		}

		logger.LastResort.Fatal("Couldn't find a default config! Please specify a config file with -c")
	}
}
