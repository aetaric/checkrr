/*
Copyright © 2022 Dustin Essington <aetaric@gmail.com>

*/
package cmd

import (
	"fmt"
	"os"
	"runtime"

	log "github.com/sirupsen/logrus"

	"github.com/common-nighthawk/go-figure"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var cfgFile string

// These vars are set at compile time by goreleaser
var version = "development"
var commit string
var date string
var builtBy string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "checkrr",
	Version: getVersion(),
	Short:   "Checks your media files and stores hashes for future checking",
	Long:    `Disks fail, bits rot... checkrr makes sure your media files are in good condition.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		injectViper(viper.GetViper(), cmd)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func getVersion() string {
	return fmt.Sprintf("%s\n Commit: %s\n Built on: %s\n Built By: %s", version, commit, date, builtBy)
}

func init() {
	ascii := figure.NewColorFigure("checkrr", "block", "green", true)
	ascii.Print()

	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "/etc/checkrr.yaml", "config file")
}

// Read explicitly set values from viper and override Flags
//  values with the same long-name if they were not explicitly set
// via cmd line
func injectViper(cmdViper *viper.Viper, cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if !f.Changed {
			if cmdViper.IsSet(f.Name) {
				cmd.Flags().Set(f.Name, cmdViper.GetString(f.Name))
			}
		}
	})
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

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
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	} else {
		log.Printf("err: %v", err.Error())
	}
}
