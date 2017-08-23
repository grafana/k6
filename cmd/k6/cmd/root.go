/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package cmd

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Version = "0.17.1"
var Banner = `
          /\      |‾‾|  /‾‾/  /‾/   
     /\  /  \     |  |_/  /  / /   
    /  \/    \    |      |  /  ‾‾\  
   /          \   |  |‾\  \ | (_) | 
  / __________ \  |__|  \__\ \___/ .io`

var cfgFile string

// RootCmd represents the base command when called without any subcommands.
var RootCmd = &cobra.Command{
	Use:   "k6",
	Short: "a next-generation load generator",
	Long:  Banner,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if viper.GetBool("verbose") {
			log.SetLevel(log.DebugLevel)
		}
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().BoolP("verbose", "v", false, "enable debug logging")
	if err := viper.BindPFlags(RootCmd.PersistentFlags()); err != nil {
		panic(err)
	}

	// It makes no sense to bind this to viper, so register it afterwards.
	RootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default ./k6.yaml ~/.config/k6.yaml)")
	cobra.MarkFlagFilename(RootCmd.PersistentFlags(), "config")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.AddConfigPath(".")             // Look for config files in the current directory.
	viper.AddConfigPath("$HOME/.config") // Look for config files in $HOME/.config.
	viper.SetConfigName("k6")            // Look for config files named "k6.yaml".
	viper.SetConfigType("yaml")          // Look for config files named "k6.yaml".
	viper.SetConfigFile(cfgFile)         // Let -c/--config override config path.
	viper.SetEnvPrefix("k6")             // Read environment variables starting with "K6_".

	// Auto-load matching environment vars, eg. K6_VERBOSE -> verbose.
	viper.AutomaticEnv()

	// Find a config file and load it.
	if err := viper.ReadInConfig(); err != nil {
		_, isNotFound := err.(viper.ConfigFileNotFoundError)
		if cfgFile != "" || !isNotFound {
			log.WithError(err).Error("Couldn't read config")
		}
	}
}
