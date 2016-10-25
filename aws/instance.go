package aws

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

// Instance is a is a Reapable, Filterable
// embeds AWS API's ec2.Instance
type Instance struct {
	Resource
	ec2.Instance
	SecurityGroups map[reapable.ID]string
	AutoScaled     bool
}

// NewInstance creates an Instance from the AWS API's ec2.Instance
func NewInstance(region string, instance *ec2.Instance) *Instance {
	a := Instance{
		Resource: Resource{
			ResourceType: "Instance",
			id:           reapable.ID(*instance.InstanceId),
			region:       reapable.Region(region), // passed in cause not possible to extract out of api
			Tags:         make(map[string]string),
		},
		SecurityGroups: make(map[reapable.ID]string),
		Instance:       *instance,
	}

	for _, sg := range instance.SecurityGroups {
		if sg != nil {
			a.SecurityGroups[reapable.ID(*sg.GroupId)] = *sg.GroupName
		}
	}

	for _, tag := range instance.Tags {
		a.Resource.Tags[*tag.Key] = *tag.Value
	}

	if a.Tagged("aws:cloudformation:stack-name") {
		a.Dependency = true
		a.IsInCloudformation = true
	}

	if a.Tagged("aws:autoscaling:groupName") {
		a.Dependency = true
		a.AutoScaled = true
	}

	a.Name = a.Tag("Name")

	if a.Tagged(reaperTag) {
		// restore previously tagged state
		a.reaperState = state.NewStateWithTag(a.Tag(reaperTag))
	} else {
		// initial state
		a.reaperState = state.NewState()
	}

	return &a
}

// Pending returns whether an instance's State is Pending
func (a *Instance) Pending() bool { return *a.State.Code == 0 }

// Running returns whether an instance's State is Running
func (a *Instance) Running() bool { return *a.State.Code == 16 }

// ShuttingDown returns whether an instance's State is ShuttingDown
func (a *Instance) ShuttingDown() bool { return *a.State.Code == 32 }

// Terminated returns whether an instance's State is Terminated
func (a *Instance) Terminated() bool { return *a.State.Code == 48 }

// Stopping returns whether an instance's State is Stopping
func (a *Instance) Stopping() bool { return *a.State.Code == 64 }

// Stopped returns whether an instance's State is Stopped
func (a *Instance) Stopped() bool { return *a.State.Code == 80 }

// AWSConsoleURL returns the url that can be used to access the resource on the AWS Console
func (a *Instance) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/v2/home?region=%s#Instances:instanceId=%s",
		a.Region().String(), a.Region().String(), url.QueryEscape(a.ID().String())))
	if err != nil {
		log.Error("Error generating AWSConsoleURL. ", err)
	}
	return url
}

func (a *Instance) Filter(filter filters.Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
	case "State":
		if a.State != nil && *a.State.Name == filter.Arguments[0] {
			matched = true
		}
	case "InstanceType":
		if a.InstanceType != nil && *a.InstanceType == filter.Arguments[0] {
			matched = true
		}
	case "HasPublicIpAddress":
		if b, err := filter.BoolValue(0); err == nil && b == (a.PublicIpAddress != nil) {
			matched = true
		}
	case "PublicIpAddress":
		if a.PublicIpAddress != nil && *a.PublicIpAddress == filter.Arguments[0] {
			matched = true
		}
	case "InCloudformation":
		if b, err := filter.BoolValue(0); err == nil && a.IsInCloudformation == b {
			matched = true
		}
	case "AutoScaled":
		if b, err := filter.BoolValue(0); err == nil && a.AutoScaled == b {
			matched = true
		}
	// uses RFC3339 format
	// https://www.ietf.org/rfc/rfc3339.txt
	case "LaunchTimeBefore":
		t, err := time.Parse(time.RFC3339, filter.Arguments[0])
		if err == nil && a.LaunchTime != nil && t.After(*a.LaunchTime) {
			matched = true
		}
	case "LaunchTimeAfter":
		t, err := time.Parse(time.RFC3339, filter.Arguments[0])
		if err == nil && a.LaunchTime != nil && t.Before(*a.LaunchTime) {
			matched = true
		}
	case "LaunchTimeInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && a.LaunchTime != nil && time.Since(*a.LaunchTime) < d {
			matched = true
		}
	case "LaunchTimeNotInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && a.LaunchTime != nil && time.Since(*a.LaunchTime) > d {
			matched = true
		}
	case "Region":
		for _, region := range filter.Arguments {
			if a.Region() == reapable.Region(region) {
				matched = true
			}
		}
	case "NotRegion":
		// was this resource's region one of those in the NOT list
		regionSpecified := false
		for _, region := range filter.Arguments {
			if a.Region() == reapable.Region(region) {
				regionSpecified = true
			}
		}
		if !regionSpecified {
			matched = true
		}
	case "Tagged":
		if a.Tagged(filter.Arguments[0]) {
			matched = true
		}
	case "NotTagged":
		if !a.Tagged(filter.Arguments[0]) {
			matched = true
		}
	case "TagNotEqual":
		if a.Tag(filter.Arguments[0]) != filter.Arguments[1] {
			matched = true
		}
	case "ReaperState":
		if a.reaperState.State.String() == filter.Arguments[0] {
			matched = true
		}
	case "NotReaperState":
		if a.reaperState.State.String() != filter.Arguments[0] {
			matched = true
		}
	case "Named":
		if a.Name == filter.Arguments[0] {
			matched = true
		}
	case "NotNamed":
		if a.Name != filter.Arguments[0] {
			matched = true
		}
	case "IsDependency":
		if b, err := filter.BoolValue(0); err == nil && a.Dependency == b {
			matched = true
		}
	case "NameContains":
		if strings.Contains(a.Name, filter.Arguments[0]) {
			matched = true
		}
	case "NotNameContains":
		if !strings.Contains(a.Name, filter.Arguments[0]) {
			matched = true
		}
	default:
		log.Error(fmt.Sprintf("No function %s could be found for filtering Instances.", filter.Function))
	}
	return matched
}

// Terminate is a method of reapable.Terminable, which is embedded in reapable.Reapable
func (a *Instance) Terminate() (bool, error) {
	log.Info("Terminating Instance %s", a.ReapableDescriptionTiny())
	api := ec2.New(sess, aws.NewConfig().WithRegion(a.Region().String()))
	req := &ec2.TerminateInstancesInput{
		InstanceIds: []*string{aws.String(a.ID().String())},
	}

	resp, err := api.TerminateInstances(req)

	if err != nil {
		return false, err
	}

	if len(resp.TerminatingInstances) != 1 {
		return false, fmt.Errorf("Instance could %s not be terminated.", a.ReapableDescriptionTiny())
	}

	return true, nil
}

// Start starts an instance
func (a *Instance) Start() (bool, error) {
	log.Info("Starting Instance %s", a.ReapableDescriptionTiny())
	api := ec2.New(sess, aws.NewConfig().WithRegion(string(a.Region())))
	req := &ec2.StartInstancesInput{
		InstanceIds: []*string{aws.String(a.ID().String())},
	}

	resp, err := api.StartInstances(req)

	if err != nil {
		return false, err
	}

	if len(resp.StartingInstances) != 1 {
		return false, fmt.Errorf("Instance %s could not be started.", a.ReapableDescriptionTiny())
	}

	return true, nil
}

// Stop is a method of reapable.Stoppable, which is embedded in reapable.Reapable
func (a *Instance) Stop() (bool, error) {
	log.Info("Stopping Instance %s", a.ReapableDescriptionTiny())
	api := ec2.New(sess, aws.NewConfig().WithRegion(string(a.Region())))
	req := &ec2.StopInstancesInput{
		InstanceIds: []*string{aws.String(a.ID().String())},
	}

	resp, err := api.StopInstances(req)

	if err != nil {
		return false, err
	}

	if len(resp.StoppingInstances) != 1 {
		return false, fmt.Errorf("Instance %s could not be stopped.", a.ReapableDescriptionTiny())
	}

	return true, nil
}
