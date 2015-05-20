package reaper

import (
	"fmt"
	"net/mail"
	"strings"
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

type StateEnum int

const (
	STATE_START StateEnum = iota
	STATE_NOTIFY1
	STATE_NOTIFY2
	STATE_IGNORE
)

func (s StateEnum) String() string {
	switch s {
	case STATE_NOTIFY1:
		return "notify1"
	case STATE_NOTIFY2:
		return "notify2"
	case STATE_IGNORE:
		return "ignore"
	default:
		return "start"
	}
}

const (
	reaper_tag = "REAPER"
	s_sep      = "|"
	s_tformat  = "2006-01-02 03:04PM MST"
)

type State struct {
	State StateEnum

	// State must be maintained until this time
	Until time.Time
}

func (s *State) String() string {
	return s.State.String() + s_sep + s.Until.Format(s_tformat)
}

type Instances []*Instance
type Instance struct {
	id         string
	region     string
	state      string
	launchTime time.Time

	tags map[string]string

	// reaper state
	reaper *State
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
		id:         _id,
		region:     region, // passed in cause not possible to extract out of api
		state:      _state,
		launchTime: _launch,
		tags:       make(map[string]string),
	}

	for _, tag := range instance.Tags {
		i.tags[*tag.Key] = *tag.Value
	}

	i.reaper = ParseState(i.tags[reaper_tag])

	return &i
}

func (i *Instance) Tagged(tag string) (ok bool) {
	_, ok = i.tags[tag]
	return
}

func (i *Instance) Id() string            { return i.id }
func (i *Instance) Region() string        { return i.region }
func (i *Instance) State() string         { return i.state }
func (i *Instance) LaunchTime() time.Time { return i.launchTime }
func (i *Instance) Reaper() *State        { return i.reaper }

// Name extracts the "Name" tag
func (i *Instance) Name() string { return i.tags["Name"] }

// Owned checks if the instance has an Owner tag
func (i *Instance) Owned() (ok bool) { return i.Tagged("Owner") }

// Owner extracts useful information out of the Owner tag which should
// be parsable by mail.ParseAddress
func (i *Instance) Owner() *mail.Address {
	if addr, err := mail.ParseAddress(i.Tag("Owner")); err == nil {
		return addr
	}

	return nil
}

// Tag returns the tag's value or an empty string if it does not exist
func (i *Instance) Tag(t string) string { return i.tags[t] }

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

func ParseState(state string) (defaultState *State) {

	defaultState = &State{STATE_START, time.Time{}}

	if state == "" {
		return
	}

	s := strings.Split(state, s_sep)

	if len(s) != 2 {
		return
	}

	var stateEnum StateEnum
	switch s[0] {
	case "start":
		stateEnum = STATE_START
	case "notify1":
		stateEnum = STATE_NOTIFY1
	case "notify2":
		stateEnum = STATE_NOTIFY2
	case "ignore":
		stateEnum = STATE_IGNORE
	default:
		return
	}

	t, err := time.Parse(s_tformat, s[1])
	if err != nil {
		return
	}

	return &State{stateEnum, t}
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
