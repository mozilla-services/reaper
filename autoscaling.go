package main

import (
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/service/autoscaling"
)

type AutoScalingGroup struct {
	AWSResource

	// autoscaling.Instance exposes minimal info
	instances []string

	autoScalingGroupARN     string
	createdTime             time.Time
	maxSize                 int64
	minSize                 int64
	size                    int64
	launchConfigurationName string
}

func NewAutoScalingGroup(region string, asg *autoscaling.Group) *AutoScalingGroup {
	a := AutoScalingGroup{
		AWSResource: AWSResource{
			name:   *asg.AutoScalingGroupName,
			region: region,
			tags:   make(map[string]string),
		},
		autoScalingGroupARN: *asg.AutoScalingGroupARN,
		createdTime:         *asg.CreatedTime,
		maxSize:             *asg.MaxSize,
		minSize:             *asg.MinSize,
		size:                *asg.DesiredCapacity,
		launchConfigurationName: *asg.LaunchConfigurationName,
	}

	for i := 0; i < len(asg.Instances); i++ {
		a.instances = append(a.instances, *asg.Instances[i].InstanceID)
	}

	for i := 0; i < len(asg.Tags); i++ {
		a.tags[*asg.Tags[i].Key] = *asg.Tags[i].Value
	}

	return &a
}

func SizeGreaterThan(a *AutoScalingGroup, size int64) bool {
	return a.size >= size
}

func (a *AutoScalingGroup) Filter(filter Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
	case "SizeGreaterThan":
		i, err := strconv.ParseInt(filter.Value, 10, 64)
		if err != nil {
			Log.Error("could not parse %s as int64", filter.Value)
		}
		if SizeGreaterThan(a, i) {
			matched = true
		}
	default:
		Log.Error("No function %s could be found for filtering ASGs.", filter.Function)
	}
	return matched
}

// TODO
func (a *AutoScalingGroup) Terminate() (bool, error) {
	return false, nil
}

// TODO
func (a *AutoScalingGroup) Stop() (bool, error) {
	return false, nil
}
