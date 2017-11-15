package cmd

import (
	"fmt"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var mappingsFile string

// ConfigData holds the basic configuration needed for ASG fetching
type ConfigData struct {
	Regions  []string
	Commands map[string]string
	Mappings map[string]string
}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "asgmatic",
	Short: "Queries and generates upgrade commands for AWS ASGs",
	Long: `ASG-Matic is a tool to automate some tedious tasks with
AWS management. It aims to be self contained and easily
manageable.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.asgmatic.yaml)")

	RootCmd.PersistentFlags().StringVar(&mappingsFile, "mappings-file", "mappings.yaml", "Location for mappings file")
	viper.BindPFlag("mappings", RootCmd.PersistentFlags().Lookup("mappings-file"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".asgmatic" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".asgmatic")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
