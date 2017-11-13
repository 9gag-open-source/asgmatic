package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/labstack/gommon/log"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type asg struct {
	Region, Name, LaunchConfig, InstanceType, AmiID, NewAmiID, NewAmiName string
}

func resolveAsg(svc *autoscaling.AutoScaling, region, q string) ([]asg, error) {
	var input *autoscaling.DescribeAutoScalingGroupsInput
	if q != "" {
		input = &autoscaling.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: []*string{&q},
		}
	}

	result, err := svc.DescribeAutoScalingGroups(input)
	if err != nil {
		return nil, errors.Wrap(err, "unable to describe autoscaling groups")
	}

	var asgs []asg

	for _, a := range result.AutoScalingGroups {
		asgs = append(asgs, asg{Region: region, Name: *a.AutoScalingGroupName, LaunchConfig: *a.LaunchConfigurationName})
	}

	return asgs, nil
}

func failAndExit(err error, message string) {
	if err != nil {
		log.Errorf("%s: %v", message, err)
		os.Exit(1)
	}
}

type configData struct {
	Regions  []string
	Mappings map[string]string
}

func main() {
	var mappings configData

	contents, err := ioutil.ReadFile("maps.yaml")
	failAndExit(err, "unable to read mappings")

	err = yaml.Unmarshal(contents, &mappings)
	failAndExit(err, "failed to parse mappings yaml")

	for _, region := range mappings.Regions {
		sess := session.Must(session.NewSession(aws.NewConfig().WithRegion(region)))
		svc := autoscaling.New(sess)

		a, err := resolveAsg(svc, region, "")
		failAndExit(err, "unable to resolve autoscaling groups")

		err = resolveLaunchConfig(svc, launchConfigToResolve(&a))
		failAndExit(err, "unable to resolve AMI and Instance info")

		v := launchConfigToResolve(&a)
		if len(v) > 0 {
			for _, lc := range v {
				fmt.Printf("Failed to resolve %s (lc %s)\n", lc.Name, lc.LaunchConfig)
			}
		}

		for i, obj := range a {
			if n, ok := mappings.Mappings[obj.AmiID]; ok {
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
}

func resolveAmiNames(svc *ec2.EC2, a []*asg) error {
	cache := newCache()

	for _, i := range a {
		n, err := resolveAmiName(svc, &cache, i.NewAmiID)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to resolve ami %s", i.NewAmiID))
		}
		i.NewAmiName = n
	}

	return nil
}

func resolveAmiName(svc *ec2.EC2, cache *amiCache, q string) (string, error) {
	if cached := cache.Get(q); cached != "" {
		return cached, nil
	}

	input := ec2.DescribeImagesInput{ImageIds: []*string{&q}}

	res, err := svc.DescribeImages(&input)
	if err != nil {
		return "", errors.Wrap(err, "unable to fetch ami info")
	}

	if len(res.Images) == 0 {
		return "", fmt.Errorf("image not found")
	}

	// Cache all results regardless if there are more than one result
	for _, image := range res.Images {
		cache.Set(*image.ImageId, *image.Name)
	}

	return cache.Get(q), nil
}

func amiNamesToResolve(a *[]asg) []*asg {
	var toResolve []*asg

	for i, lc := range *a {
		if lc.NewAmiID != "" && lc.NewAmiName == "" {
			toResolve = append(toResolve, &(*a)[i])
		}
	}

	return toResolve
}

func launchConfigToResolve(a *[]asg) []*asg {
	var toResolve []*asg

	for i, lc := range *a {
		if lc.AmiID == "" {
			toResolve = append(toResolve, &(*a)[i])
		}
	}

	return toResolve
}

func resolveLaunchConfig(svc *autoscaling.AutoScaling, toResolve []*asg) error {
	r := make(map[string][]*asg)
	var names []*string

	for _, a := range toResolve {
		names = append(names, &a.LaunchConfig)
		l := r[a.LaunchConfig]
		r[a.LaunchConfig] = append(l, a)
	}

	results, err := getLaunchConfigurations(svc, names)
	if err != nil {
		return errors.Wrap(err, "unable to resolve launch configurations from AWS")
	}

	for _, lc := range results {
		as, ok := r[*lc.LaunchConfigurationName]
		if !ok {
			fmt.Printf("Attempted to find %s but failed\n", *lc.LaunchConfigurationName)
			// If the key isn't found, simply skip the key
			continue
		}

		for _, a := range as {
			a.AmiID = *lc.ImageId
			a.InstanceType = *lc.InstanceType
		}
	}

	return nil
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

type amiCache struct{ cache map[string]string }

func newCache() amiCache                  { return amiCache{cache: make(map[string]string)} }
func (c *amiCache) Get(key string) string { return c.cache[key] }
func (c *amiCache) Set(key, value string) { c.cache[key] = value }
