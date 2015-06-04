package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go/aws"
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
			id:     *asg.AutoScalingGroupName,
			name:   *asg.AutoScalingGroupName,
			region: region,
			tags:   make(map[string]string),
			reaper: ParseState(""),
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

func (a *AutoScalingGroup) SizeGreaterThanOrEqualTo(size int64) bool {
	return a.size >= size
}

func (a *AutoScalingGroup) SizeLessThanOrEqualTo(size int64) bool {
	return a.size <= size
}

func (a *AutoScalingGroup) SizeEqualTo(size int64) bool {
	return a.size == size
}

func (a *AutoScalingGroup) SizeLessThan(size int64) bool {
	return a.size < size
}

func (a *AutoScalingGroup) SizeGreaterThan(size int64) bool {
	return a.size <= size
}

func (a *AutoScalingGroup) Filter(filter Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
	case "SizeGreaterThan":
		if i, err := filter.Int64Value(); err == nil && a.SizeGreaterThan(i) {
			matched = true
		}
	case "SizeLessThan":
		if i, err := filter.Int64Value(); err == nil && a.SizeLessThan(i) {
			matched = true
		}
	case "SizeEqualTo":
		if i, err := filter.Int64Value(); err == nil && a.SizeEqualTo(i) {
			matched = true
		}
	case "SizeLessThanOrEqualTo":
		if i, err := filter.Int64Value(); err == nil && a.SizeLessThanOrEqualTo(i) {
			matched = true
		}
	case "SizeGreaterThanOrEqualTo":
		if i, err := filter.Int64Value(); err == nil && a.SizeGreaterThanOrEqualTo(i) {
			matched = true
		}
	case "Tagged":
		if a.Tagged(filter.Value) {
			matched = true
		}
	default:
		Log.Error("No function %s could be found for filtering ASGs.", filter.Function)
	}
	return matched
}

func (a *AutoScalingGroup) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/autoscaling/home?region=%s#AutoScalingGroups:id=%s",
		a.Region(), a.Region(), a.Id()))
	if err != nil {
		Log.Error(fmt.Sprintf("Error generating AWSConsoleURL. %s", err))
	}
	return url
}

// TODO
func (a *AutoScalingGroup) Terminate() (bool, error) {
	Log.Debug("Terminating ASG %s in region %s.", a.id, a.region)
	return false, nil
}

// stopping an ASG == scaling it to 0
func (a *AutoScalingGroup) Stop() (bool, error) {
	Log.Debug("Stopping ASG %s in region %s", a.Id(), a.Region())
	as := autoscaling.New(&aws.Config{Region: a.Region()})

	// TODO: fix this nonsense
	zero := int64(0)

	input := &autoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: &a.id,
		DesiredCapacity:      &zero,
	}
	_, err := as.SetDesiredCapacity(input)
	if err != nil {
		Log.Error("could not set desired capacity to 0 for ASG %s in region %s", a.Id(), a.Region())
		return false, err
	}
	return true, nil
}

// stopping an ASG == scaling it to 0
func (a *AutoScalingGroup) ForceStop() (bool, error) {
	Log.Debug("Force Stopping ASG %s in region %s", a.Id(), a.Region())
	as := autoscaling.New(&aws.Config{Region: a.Region()})

	// TODO: fix this nonsense
	zero := int64(0)

	input := &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: &a.id,
		DesiredCapacity:      &zero,
		MinSize:              &zero,
	}
	_, err := as.UpdateAutoScalingGroup(input)
	if err != nil {
		Log.Error("could not set DesiredCapacity, MinSize to 0 for ASG %s in region %s", a.Id(), a.Region())
		return false, err
	}
	return true, nil
}
