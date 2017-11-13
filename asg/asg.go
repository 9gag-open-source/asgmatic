package asg

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
)

// Asg is used to collect data from AWS
type Asg struct {
	Region, Name, LaunchConfig, InstanceType, AmiID, NewAmiID, NewAmiName string
}

// ConfigData holds the basic configuration needed for ASG fetching
type ConfigData struct {
	Regions  []string
	Mappings map[string]string
}

// GenerateASGTemplates reads ASG information from AWS and prints out commands to for upgrading
// those ASG configurations to the latest one
func GenerateASGTemplates(config ConfigData) error {
	for _, region := range config.Regions {
		sess := session.Must(session.NewSession(aws.NewConfig().WithRegion(region)))
		svc := autoscaling.New(sess)

		a, err := resolveAsg(svc, region, "")
		if err != nil {
			return errors.Wrap(err, "unable to resolve autoscaling groups")
		}

		err = resolveLaunchConfig(svc, launchConfigToResolve(&a))
		if err != nil {
			return errors.Wrap(err, "unable to resolve AMI and instance info")
		}

		v := launchConfigToResolve(&a)
		if len(v) > 0 {
			for _, lc := range v {
				fmt.Printf("Failed to resolve %s (lc %s)\n", lc.Name, lc.LaunchConfig)
			}
		}

		for i, obj := range a {
			if n, ok := config.Mappings[obj.AmiID]; ok {
				a[i].NewAmiID = n
			}
		}

		ec2svc := ec2.New(sess)
		err = resolveAmiNames(ec2svc, amiNamesToResolve(&a))
		if err != nil {
			fmt.Printf("error during AMI resolving: %v\n", err)
		}

		fmt.Println(a)
	}

	return nil
}

// ReportUnknownAmis will print out AMIs not mapped to new AMIs but still found in the system
func ReportUnknownAmis(config ConfigData) error {
	amis := map[string]string{}
	cache := newCache()

	for _, region := range config.Regions {
		sess := session.Must(session.NewSession(aws.NewConfig().WithRegion(region)))
		svc := autoscaling.New(sess)

		a, err := resolveAsg(svc, region, "")
		if err != nil {
			return errors.Wrap(err, "unable to resolve autoscaling groups")
		}

		err = resolveLaunchConfig(svc, launchConfigToResolve(&a))
		if err != nil {
			return errors.Wrap(err, "unable to resolve AMI and instance info")
		}

		ec2svc := ec2.New(sess)
		for _, lc := range a {
			if _, ok := config.Mappings[lc.AmiID]; !ok {
				name, err := resolveAmiName(ec2svc, &cache, lc.AmiID)
				if err != nil {
					return errors.Wrap(err, "Error while resolving AMI names")
				}
				amis[lc.AmiID] = fmt.Sprintf("%s / %s", region, name)
			}
		}
	}

	for key, name := range amis {
		fmt.Printf("# %s: # %s\n", key, name)
	}

	return nil
}

func amiNamesToResolve(a *[]Asg) []*Asg {
	var toResolve []*Asg

	for i, lc := range *a {
		if lc.NewAmiID != "" && lc.NewAmiName == "" {
			toResolve = append(toResolve, &(*a)[i])
		}
	}

	return toResolve
}

func launchConfigToResolve(a *[]Asg) []*Asg {
	var toResolve []*Asg

	for i, lc := range *a {
		if lc.AmiID == "" {
			toResolve = append(toResolve, &(*a)[i])
		}
	}

	return toResolve
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func getLaunchConfigurations(svc *autoscaling.AutoScaling, names []*string) ([]*autoscaling.LaunchConfiguration, error) {
	if len(names) == 0 {
		return nil, nil
	}

	batchlen := min(len(names), 50)
	names, queue := names[:batchlen], names[batchlen:]
	lcinput := &autoscaling.DescribeLaunchConfigurationsInput{LaunchConfigurationNames: names}

	lcresult, err := svc.DescribeLaunchConfigurations(lcinput)
	if err != nil {
		return nil, errors.Wrap(err, "unable to fetch launch configurations")
	}

	qlcs, err := getLaunchConfigurations(svc, queue)
	if err != nil {
		return nil, err
	}

	return append(lcresult.LaunchConfigurations, qlcs...), nil
}
