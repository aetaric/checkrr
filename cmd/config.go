/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Outputs a default config",
	Long:  `Generates a default config and places it in the specified (or default) path`,
	Run: func(cmd *cobra.Command, args []string) {

		viper.GetViper().WriteConfigAs(cfgFile)
		log.Printf("Saved default config file to: %v", cfgFile)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
