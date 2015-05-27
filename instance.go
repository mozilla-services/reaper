package reaper

import (
	"fmt"
	"time"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/service/ec2"
	"github.com/mostlygeek/reaper/filter"
	. "github.com/tj/go-debug"
)

var (
	debugAWS = Debug("reaper:aws")
	debugAll = Debug("reaper:aws:AllInstances")
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
}

func NewInstance(region string, instance *ec2.Instance) *Instance {

	// ughhhhhh pointers to strings suck
	_id := "nil"
	_state := "nil"
	var _launch time.Time

	if instance.InstanceID != nil {
		_id = *instance.InstanceID
	}

	if instance.State != nil {
		if instance.State.Name != nil {
			_state = *instance.State.Name
		}
	}

	if instance.LaunchTime != nil {
		_launch = *instance.LaunchTime
	} else {
		_launch = time.Time{}
	}

	i := Instance{
		AWSResource: AWSResource{
			id:     _id,
			region: region, // passed in cause not possible to extract out of api
			state:  _state,
			tags:   make(map[string]string),
		},

		securityGroups: make(map[string]string),
		launchTime:     _launch,
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

func (i *Instance) LaunchTime() time.Time { return i.launchTime }

// Autoscaled checks if the instance is part of an autoscaling group
func (i *Instance) AutoScaled() (ok bool) { return i.Tagged("aws:autoscaling:groupName") }

func (i *Instance) ReaperVisible() bool {
	return time.Now().After(i.reaper.Until)
}
func (i *Instance) ReaperStarted() bool {
	return i.reaper.State == STATE_START
}
func (i *Instance) ReaperNotified(notifyNum int) bool {
	if notifyNum == 1 {
		return i.reaper.State == STATE_NOTIFY1
	} else if notifyNum == 2 {
		return i.reaper.State == STATE_NOTIFY2
	} else {
		return false
	}
}

func (i *Instance) ReaperIgnored() bool {
	return i.reaper.State == STATE_IGNORE
}

func UpdateReaperState(region, instanceId string, newState *State) error {
	debugAWS("UpdateReaperState region:%s instance: %s", region, instanceId)
	req := &ec2.CreateTagsInput{
		DryRun:    aws.Boolean(false),
		Resources: []*string{aws.String(instanceId)},
		Tags: []*ec2.Tag{
			&ec2.Tag{
				Key:   aws.String(reaper_tag),
				Value: aws.String(newState.String()),
			},
		},
	}

	api := ec2.New(&aws.Config{Region: region})
	_, err := api.CreateTags(req)
	return err
}

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
