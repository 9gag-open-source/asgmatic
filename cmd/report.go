package cmd

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"

	"github.com/sami9gag/asgmatic/asg"
)

// reportCmd represents the report command
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate report of unmapped AMIs",
	Long: `Walk all in use launch configurations and report AMIs
that are in use and don't have a mapping in mappings file.`,
	Run: func(cmd *cobra.Command, args []string) {
		var mappings ConfigData

		contents, err := ioutil.ReadFile(viper.GetString("mappingsFile"))
		if err != nil {
			fmt.Printf("unable to read mappings: %v\n", err)
			os.Exit(1)
		}

		err = yaml.Unmarshal(contents, &mappings)
		if err != nil {
			fmt.Printf("failed to parse mappings yaml: %v\n", err)
			os.Exit(1)
		}

		for _, region := range mappings.Regions {
			err = asg.ReportUnknownAmis(region, mappings.Mappings, os.Stdout)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(reportCmd)
}
