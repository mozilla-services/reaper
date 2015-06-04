package main

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const (
	reaper_tag = "REAPER"
	s_sep      = "|"
	s_tformat  = "2006-01-02 03:04PM MST"
)

type Instance struct {
	AWSResource
	launchTime     time.Time
	securityGroups map[string]string
}

func NewInstance(region string, instance *ec2.Instance) *Instance {
	i := Instance{
		AWSResource: AWSResource{
			id:     *instance.InstanceID,
			region: region, // passed in cause not possible to extract out of api
			tags:   make(map[string]string),
		},

		securityGroups: make(map[string]string),
		launchTime:     *instance.LaunchTime,
	}

	for _, sg := range instance.SecurityGroups {
		i.securityGroups[*sg.GroupID] = *sg.GroupName
	}

	for _, tag := range instance.Tags {
		i.tags[*tag.Key] = *tag.Value
	}

	switch *instance.State.Code {
	case 0:
		i.state = PENDING
	case 16:
		i.state = RUNNING
	case 32:
		i.state = SHUTTINGDOWN
	case 48:
		i.state = TERMINATED
	case 64:
		i.state = STOPPING
	case 80:
		i.state = STOPPED
	}

	i.name = i.Tag("Name")
	i.reaper = ParseState(i.tags[reaper_tag])

	return &i
}

func (i *Instance) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/v2/home?region=%s#Instances:instanceId=%s",
		i.region, i.region, i.id))
	if err != nil {
		Log.Error(fmt.Sprintf("Error generating AWSConsoleURL. %s", err))
	}
	return url
}

func (i *Instance) LaunchTime() time.Time { return i.launchTime }

// Autoscaled checks if the instance is part of an autoscaling group
func (i *Instance) AutoScaled() (ok bool) { return i.Tagged("aws:autoscaling:groupName") }

func (i *Instance) Filter(filter Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
	case "Running":
		b, err := strconv.ParseBool(filter.Value)
		if err != nil {
			Log.Error("could not parse %s as bool", filter.Value)
		}
		if i.Running() == b {
			matched = true
		}
	case "Tagged":
		if i.Tagged(filter.Value) {
			matched = true
		}
	default:
		Log.Error("No function %s could be found for filtering ASGs.", filter.Function)
	}
	return matched
}

func Whitelist(region, instanceId string) error {
	api := ec2.New(&aws.Config{Region: region})
	req := &ec2.CreateTagsInput{
		Resources: []*string{aws.String(instanceId)},
		Tags: []*ec2.Tag{
			&ec2.Tag{
				Key:   aws.String("REAPER_SPARE_ME"),
				Value: aws.String("true"),
			},
		},
	}

	_, err := api.CreateTags(req)

	if err != nil {
		return err
	}

	return nil
}

func (i *Instance) Terminate() (bool, error) {
	api := ec2.New(&aws.Config{Region: i.region})
	req := &ec2.TerminateInstancesInput{
		InstanceIDs: []*string{aws.String(i.id)},
	}

	resp, err := api.TerminateInstances(req)

	if err != nil {
		return false, err
	}

	if len(resp.TerminatingInstances) != 1 {
		return false, fmt.Errorf("Instance could %s not be terminated.", i.id)
	}

	return true, nil
}

func (i *Instance) ForceStop() (bool, error) {
	return i.Stop()
}

func (i *Instance) Stop() (bool, error) {
	api := ec2.New(&aws.Config{Region: i.region})
	req := &ec2.StopInstancesInput{
		InstanceIDs: []*string{aws.String(i.id)},
	}

	resp, err := api.StopInstances(req)

	if err != nil {
		return false, err
	}

	if len(resp.StoppingInstances) != 1 {
		return false, fmt.Errorf("Instance %s could not be stopped.", i.id)
	}

	return true, nil
}
