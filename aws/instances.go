package aws

import (
	"fmt"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/ec2"
	. "github.com/tj/go-debug"
)

var (
	debug    = Debug("reaper:aws")
	debugAll = Debug("reaper:aws:AllInstances")
)

type FilterFunc func(*Instance) bool

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
	REAPER_TAG = "REAPER"
	S_SEP      = "|"
	S_TFORMAT  = "2006-01-02 03:04PM MST"
)

type State struct {
	State StateEnum

	// State must be maintained until this time
	Until time.Time
}

func (s *State) Visible() bool { return time.Now().After(s.Until) }
func (s *State) String() string {
	return s.State.String() + S_SEP + s.Until.Format(S_TFORMAT)
}

type Instances []*Instance
type Instance struct {
	id         string
	api        *ec2.EC2
	instance   *ec2.Instance
	region     string
	state      string
	launchTime time.Time

	tags map[string]string

	// reaper state
	reaper *State
}

func NewInstance(region string, api *ec2.EC2, instance *ec2.Instance) *Instance {

	// ughhhhhh pointers to strings suck
	_id := "nil"
	_state := "nil"

	if instance.InstanceID != nil {
		_id = *instance.InstanceID
	}

	if instance.State != nil {
		if instance.State.Name != nil {
			_state = *instance.State.Name
		}
	}

	i := Instance{
		id:         _id,
		region:     region, // passed in cause not possible to extract out of api
		api:        api,
		state:      _state,
		launchTime: instance.LaunchTime,
		tags:       make(map[string]string),
	}

	for _, tag := range instance.Tags {
		i.tags[*tag.Key] = *tag.Value
	}

	i.reaper = ParseState(i.tags[REAPER_TAG])

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

// state transitions

func (i *Instance) UpdateReaperState(newState *State) error {

	i.reaper = newState

	req := &ec2.CreateTagsRequest{
		DryRun:    aws.False(),
		Resources: []string{i.Id()},
		Tags: []ec2.Tag{
			ec2.Tag{
				Key:   aws.String(REAPER_TAG),
				Value: aws.String(i.Reaper().String()),
			},
		},
	}

	return i.api.CreateTags(req)
}

func (i *Instance) Ignore(until time.Time) error {
	return nil
}

func (i *Instance) Terminate() error {
	req := &ec2.TerminateInstancesRequest{
		InstanceIDs: []string{i.Id()},
	}

	resp, err := i.api.TerminateInstances(req)

	if err != nil {
		return err
	}

	if len(resp.TerminatingInstances) != 1 {
		return fmt.Errorf("Instance could not be terminated")
	}

	return nil

}

func ParseState(state string) (defaultState *State) {

	defaultState = &State{STATE_START, time.Time{}}

	if state == "" {
		return
	}

	s := strings.Split(state, S_SEP)

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

	t, err := time.Parse(S_TFORMAT, s[1])
	if err != nil {
		return
	}

	return &State{stateEnum, t}
}

// Filter creates a new list of Instances that match the filter
func (i Instances) Filter(f FilterFunc) (newList Instances) {
	for _, i := range i {
		if f(i) {
			newList = append(newList, i)
		}
	}

	return
}

// AllInstances fetches in parallel all instances in the provided endpoints
func AllInstances(endpoints EndpointMap) Instances {

	var wg sync.WaitGroup
	in := make(chan *Instance)

	// fetch all info in parallel
	for region, api := range endpoints {
		debugAll("DescribeInstances in %s", region)
		wg.Add(1)
		go func(region string, api *ec2.EC2) {
			defer wg.Done()

			resp, err := api.DescribeInstances(nil)
			if err != nil {
				// probably should do something here...
				return
			}

			sum := 0
			for _, r := range resp.Reservations {
				for _, instance := range r.Instances {
					sum += 1
					in <- NewInstance(region, api, &instance)
				}
			}

			debugAll("Found %d instances in %s", sum, region)

		}(region, api)
	}

	var list Instances
	done := make(chan struct{})

	// build up the list
	go func() {
		for i := range in {
			list = append(list, i)
		}
		done <- struct{}{}
	}()

	// wait for all the fetches to finish publishing
	wg.Wait()
	close(in)

	// wait for appending goroutine to be done
	<-done

	return list
}
