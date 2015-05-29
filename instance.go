package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/service/ec2"
	"github.com/mostlygeek/reaper/filter"
)

const (
	reaper_tag = "REAPER"
	s_sep      = "|"
	s_tformat  = "2006-01-02 03:04PM MST"
)

type Instances []*Instance
type Instance struct {
	AWSResource
	launchTime     time.Time
	securityGroups map[string]string
	awsConsoleURL  string
}

func NewInstance(region string, instance *ec2.Instance) *Instance {
	i := Instance{
		AWSResource: AWSResource{
			id:     *instance.InstanceID,
			region: region, // passed in cause not possible to extract out of api
			state:  *instance.State.Name,
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

func Terminate(region, instanceId string) error {
	api := ec2.New(&aws.Config{Region: region})
	req := &ec2.TerminateInstancesInput{
		InstanceIDs: []*string{aws.String(instanceId)},
	}

	resp, err := api.TerminateInstances(req)

	if err != nil {
		return err
	}

	if len(resp.TerminatingInstances) != 1 {
		return fmt.Errorf("Instance could not be terminated")
	}

	return nil
}

func Stop(region, instanceId string) error {
	api := ec2.New(&aws.Config{Region: region})
	req := &ec2.StopInstancesInput{
		InstanceIDs: []*string{aws.String(instanceId)},
	}

	resp, err := api.StopInstances(req)

	if err != nil {
		return err
	}

	if len(resp.StoppingInstances) != 1 {
		return fmt.Errorf("Instance could not be stopped")
	}

	return nil
}

// Filter creates a new list of Instances that match the filter
func (i Instances) Filter(f filter.FilterFunc) (newList Instances) {
	for _, i := range i {
		if f(i) {
			newList = append(newList, i)
		}
	}

	return
}

func (as Instances) Owned() Instances {
	var bs Instances
	for i := 0; i < len(as); i++ {
		if as[i].Owned() {
			bs = append(bs, as[i])
		}
	}
	return bs
}

func (as Instances) Tagged(tag string) Instances {
	var bs Instances
	for i := 0; i < len(as); i++ {
		if as[i].Tagged(tag) {
			bs = append(bs, as[i])
		}
	}
	return bs
}

func (as Instances) LaunchTimeBeforeOrEqual(time time.Time) Instances {
	var bs Instances
	for i := 0; i < len(as); i++ {
		if as[i].LaunchTime().Before(time) || as[i].LaunchTime().Equal(time) {
			bs = append(bs, as[i])
		}
	}
	return bs
}
