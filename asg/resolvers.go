package asg

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
)

func resolveAsg(svc *autoscaling.AutoScaling, region, q string) ([]asgInfo, error) {
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

	var asgs []asgInfo

	for _, a := range result.AutoScalingGroups {
		asgs = append(asgs, asgInfo{Region: region, Name: *a.AutoScalingGroupName, LaunchConfig: *a.LaunchConfigurationName})
	}

	return asgs, nil
}

func resolveAmiNames(svc *ec2.EC2, a []*asgInfo) error {
	cache := newCache()

	for _, i := range a {
		n, err := resolveAmiName(svc, &cache, i.AmiID)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to resolve ami %s", i.AmiID))
		}
		i.AmiName = n
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

// amiCache is a cache structure for AMI lookup in order to reduce the number of API queries needed
type amiCache struct{ cache map[string]string }

func newCache() amiCache                  { return amiCache{cache: make(map[string]string)} }
func (c *amiCache) Get(key string) string { return c.cache[key] }
func (c *amiCache) Set(key, value string) { c.cache[key] = value }

func resolveLaunchConfig(svc *autoscaling.AutoScaling, toResolve []*asgInfo) error {
	r := make(map[string][]*asgInfo)
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
			a.CurrentAmiID = *lc.ImageId
			a.InstanceType = *lc.InstanceType
		}
	}

	return nil
}

func resolveLatestAmi(amis map[string]string, amiID string) string {
	id, ok := amis[amiID]
	if !ok || id == amiID {
		return amiID
	}
	return resolveLatestAmi(amis, id)
}
