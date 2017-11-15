package asg

import (
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
)

// Asg is used to collect data from AWS
type Asg struct {
	Region, Name, LaunchConfig, InstanceType, CurrentAmiID, AmiID, AmiName string
}

// GenerateASGTemplates reads ASG information from AWS and prints out commands to for upgrading
// those ASG configurations to the latest one
func GenerateASGTemplates(region string, cmdTemplate string, amis map[string]string, output io.Writer) error {
	tpl := getTemplate("command", cmdTemplate+"\n")

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
		a[i].AmiID = resolveLatestAmi(amis, obj.CurrentAmiID)
	}

	ec2svc := ec2.New(sess)
	err = resolveAmiNames(ec2svc, amiNamesToResolve(&a))
	if err != nil {
		fmt.Printf("error during AMI resolving: %v\n", err)
	}

	for _, obj := range a {
		if obj.AmiID != obj.CurrentAmiID {
			tpl.Execute(output, obj)
		}
	}

	return nil
}

// ReportUnknownAmis will print out AMIs not mapped to new AMIs but still found in the system
func ReportUnknownAmis(region string, mappings map[string]string, output io.Writer) error {
	amis := map[string]string{}
	cache := newCache()

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
		if _, ok := mappings[lc.CurrentAmiID]; !ok {
			name, err := resolveAmiName(ec2svc, &cache, lc.CurrentAmiID)
			if err != nil {
				return errors.Wrap(err, "Error while resolving AMI names")
			}
			amis[lc.CurrentAmiID] = name
		}
	}

	for key, name := range amis {
		fmt.Fprintf(output, "# %s: # %s / %s\n", key, region, name)
	}

	return nil
}

func amiNamesToResolve(a *[]Asg) []*Asg {
	var toResolve []*Asg

	for i, lc := range *a {
		if lc.AmiID != "" && lc.AmiName == "" {
			toResolve = append(toResolve, &(*a)[i])
		}
	}

	return toResolve
}

func launchConfigToResolve(a *[]Asg) []*Asg {
	var toResolve []*Asg

	for i, lc := range *a {
		if lc.CurrentAmiID == "" {
			toResolve = append(toResolve, &(*a)[i])
		}
	}

	return toResolve
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

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
