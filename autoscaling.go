package main

import (
	"time"

	"github.com/awslabs/aws-sdk-go/service/autoscaling"
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

func NewAutoScalingGroup(region string, asg *autoscaling.AutoScalingGroup) *AutoScalingGroup {
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

func (a *AutoScalingGroup) Filter(f func() bool) bool {
	return f()
}

// TODO
func (a *AutoScalingGroup) Terminate() (bool, error) {
	return false, nil
}

// TODO
func (a *AutoScalingGroup) Stop() (bool, error) {
	return false, nil
}
