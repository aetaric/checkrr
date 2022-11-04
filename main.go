/*
Copyright © 2022 Dustin Essington <aetaric@gmail.com>
*/
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/aetaric/checkrr/check"
	"github.com/common-nighthawk/go-figure"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var scheduler *cron.Cron
var flagSet pflag.FlagSet

var cfgFile string
var checkVer bool
var oneShot bool
var debug bool

// These vars are set at compile time by goreleaser
var version string = "development"
var commit string
var date string
var builtBy string

func main() {
	// Prints the Banner
	ascii := figure.NewColorFigure("checkrr", "block", "green", true)
	ascii.Print()
	printVersion()

	// Verify ffprobe is in PATH
	_, binpatherr := exec.LookPath("ffprobe")
	if binpatherr != nil {
		log.WithFields(log.Fields{"startup": true}).Fatal("Failed to find ffprobe in your path... Please install FFProbe (typically included with the FFMPEG package) and make sure it is in your $PATH var. Exiting...")
	}

	// debug
	if debug {
		log.SetLevel(log.DebugLevel)
	}

	// Sets up flags
	initFlags()

	// Output Version if requested
	if checkVer {
		os.Exit(0)
	}

	// Reads in config file
	initConfig()

	// Setup SIGINT and SIGTERM handling
	term := make(chan os.Signal, 1)
	signal.Notify(term, syscall.SIGINT, syscall.SIGTERM)

	// Handle SIGHUP
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)

	// Channel to render time after execution
	rendertime := make(chan []string, 1)

	// Close the channels on exit
	defer func() {
		signal.Stop(term)
		signal.Stop(hup)
	}()

	// Start checkrr in run-once or daemon mode
	c := check.Checkrr{Chan: &rendertime}
	c.FromConfig(viper.GetViper().Sub("checkrr"))

	if oneShot {
		c.Run()
	} else {
		// Setup Cron runner.
		var id cron.EntryID
		scheduler = cron.New()
		id, _ = scheduler.AddJob(viper.GetViper().GetString("checkrr.cron"), &c)
		scheduler.Start()
		log.Infof("Next Run: %v", scheduler.Entry(id).Next.String())

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
				log.Infof("Next Run: %v", scheduler.Entry(id).Next.String())
			case <-hup:
				// Reload config and reinit scheduler on SIGHUP
				initConfig()
				scheduler.Remove(id)
				scheduler.Stop()
				id, _ = scheduler.AddJob(viper.GetViper().GetString("checkrr.cron"), &c)
				scheduler.Start()
				log.Info("Config reloaded!")
				log.Infof("Next Run: %v", scheduler.Entry(id).Next.String())
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

	flagSet.Parse(os.Args[1:])
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, _ := os.UserHomeDir()

		viper.AddConfigPath(home)

		os := runtime.GOOS
		switch os {
		case "windows":
			viper.AddConfigPath("C:/")
		default:
			viper.AddConfigPath("/etc")
			viper.AddConfigPath("/etc/checkrr")
		}
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName("checkrr")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		log.Infof("Using config file: %s", viper.ConfigFileUsed())
	} else {
		log.Printf("err: %v", err.Error())
	}
}
