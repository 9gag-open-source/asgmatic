package cmd

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"

	"github.com/sami9gag/asgmatic/asg"
)

// upgradeCmd represents the upgrade command
var upgradeAmiCmd = &cobra.Command{
	Use:   "upgrade-ami",
	Short: "generate upgrade commands for autoscaling groups",
	Long: `Generates upgrade commands for autoscaling groups based 
on command template in mappings file. Will traverse all
given regions and will generate commands for latest AMI
only.`,
	Run: func(cmd *cobra.Command, args []string) {
		var config ConfigData

		contents, err := ioutil.ReadFile(mappingsFile)
		if err != nil {
			fmt.Printf("unable to read mappings: %v\n", err)
			os.Exit(1)
		}

		err = yaml.Unmarshal(contents, &config)
		if err != nil {
			fmt.Printf("failed to parse mappings yaml: %v\n", err)
			os.Exit(1)
		}

		for _, region := range config.Regions {
			err = asg.GenerateASGTemplates(region, config.Commands["upgrade"], config.Mappings, os.Stdout)

			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(upgradeAmiCmd)
}
