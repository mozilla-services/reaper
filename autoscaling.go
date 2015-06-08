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
	Instances []string

	AutoScalingGroupARN     string
	CreatedTime             time.Time
	MaxSize                 int64
	MinSize                 int64
	DesiredCapacity         int64
	LaunchConfigurationName string
}

func NewAutoScalingGroup(region string, asg *autoscaling.Group) *AutoScalingGroup {
	a := AutoScalingGroup{
		AWSResource: AWSResource{
			ID:          *asg.AutoScalingGroupName,
			Name:        *asg.AutoScalingGroupName,
			Region:      region,
			Tags:        make(map[string]string),
			ReaperState: ParseState(""),
		},
		AutoScalingGroupARN:     *asg.AutoScalingGroupARN,
		CreatedTime:             *asg.CreatedTime,
		MaxSize:                 *asg.MaxSize,
		MinSize:                 *asg.MinSize,
		DesiredCapacity:         *asg.DesiredCapacity,
		LaunchConfigurationName: *asg.LaunchConfigurationName,
	}

	for i := 0; i < len(asg.Instances); i++ {
		a.Instances = append(a.Instances, *asg.Instances[i].InstanceID)
	}

	for i := 0; i < len(asg.Tags); i++ {
		a.Tags[*asg.Tags[i].Key] = *asg.Tags[i].Value
	}

	return &a
}

func (a *AutoScalingGroup) SizeGreaterThanOrEqualTo(size int64) bool {
	return a.DesiredCapacity >= size
}

func (a *AutoScalingGroup) SizeLessThanOrEqualTo(size int64) bool {
	return a.DesiredCapacity <= size
}

func (a *AutoScalingGroup) SizeEqualTo(size int64) bool {
	return a.DesiredCapacity == size
}

func (a *AutoScalingGroup) SizeLessThan(size int64) bool {
	return a.DesiredCapacity < size
}

func (a *AutoScalingGroup) SizeGreaterThan(size int64) bool {
	return a.DesiredCapacity <= size
}

func (a *AutoScalingGroup) Filter(filter Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
	case "SizeGreaterThan":
		if i, err := filter.Int64Value(0); err == nil && a.SizeGreaterThan(i) {
			matched = true
		}
	case "SizeLessThan":
		if i, err := filter.Int64Value(0); err == nil && a.SizeLessThan(i) {
			matched = true
		}
	case "SizeEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.SizeEqualTo(i) {
			matched = true
		}
	case "SizeLessThanOrEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.SizeLessThanOrEqualTo(i) {
			matched = true
		}
	case "SizeGreaterThanOrEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.SizeGreaterThanOrEqualTo(i) {
			matched = true
		}
	case "Tagged":
		if a.Tagged(filter.Arguments[0]) {
			matched = true
		}
	default:
		Log.Error("No function %s could be found for filtering ASGs.", filter.Function)
	}
	return matched
}

func (a *AutoScalingGroup) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/autoscaling/home?region=%s#AutoScalingGroups:ID=%s",
		a.Region, a.Region, a.ID))
	if err != nil {
		Log.Error(fmt.Sprintf("Error generating AWSConsoleURL. %s", err))
	}
	return url
}

// TODO
func (a *AutoScalingGroup) Terminate() (bool, error) {
	Log.Debug("Terminating ASG %s in region %s.", a.ID, a.Region)
	return false, nil
}

// stopping an ASG == scaling it to 0
func (a *AutoScalingGroup) Stop() (bool, error) {
	Log.Debug("Stopping ASG %s in region %s", a.ID, a.Region)
	as := autoscaling.New(&aws.Config{Region: a.Region})

	// TODO: fix this nonsense
	zero := int64(0)

	input := &autoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: &a.ID,
		DesiredCapacity:      &zero,
	}
	_, err := as.SetDesiredCapacity(input)
	if err != nil {
		Log.Error("could not set desired capacity to 0 for ASG %s in region %s", a.ID, a.Region)
		return false, err
	}
	return true, nil
}

// stopping an ASG == scaling it to 0
func (a *AutoScalingGroup) ForceStop() (bool, error) {
	Log.Debug("Force Stopping ASG %s in region %s", a.ID, a.Region)
	as := autoscaling.New(&aws.Config{Region: a.Region})

	// TODO: fix this nonsense
	zero := int64(0)

	input := &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: &a.ID,
		DesiredCapacity:      &zero,
		MinSize:              &zero,
	}
	_, err := as.UpdateAutoScalingGroup(input)
	if err != nil {
		Log.Error("could not set DesiredCapacity, MinSize to 0 for ASG %s in region %s", a.ID, a.Region)
		return false, err
	}
	return true, nil
}
