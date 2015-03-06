package reaper

import (
	"github.com/awslabs/aws-sdk-go/gen/ec2"
	"strings"
	"sync"
	"time"
)

type StateEnum int

func (s StateEnum) String() string {
	switch s {
	case STATE_NOTIFY1:
		return "notify1"
	case STATE_NOTIFY2:
		return "notify2"
	case STATE_DELAY:
		return "delay"
	}

	return "start"
}

const (
	STATE_START StateEnum = iota
	STATE_NOTIFY1
	STATE_NOTIFY2
	STATE_DELAY
)

type State struct {
	State   StateEnum
	Expires time.Time
}

func (s *State) Expired() bool { return time.Now().After(s.Expires) }
func (s *State) String() string {
	return s.State.String() + ":" + s.Expires.Format(time.RFC3339)
}

type Instances []*Instance
type Instance struct {
	api      *ec2.EC2
	instance *ec2.Instance
	region   string

	tags map[string]string

	state *State
}

func NewInstance(region string, api *ec2.EC2, instance *ec2.Instance) *Instance {
	i := Instance{
		region:   region, // passed in cause not possible to extract out of api
		api:      api,
		instance: instance,
		tags:     make(map[string]string),
	}

	for _, tag := range instance.Tags {
		i.tags[*tag.Key] = *tag.Value
	}

	i.state = ParseState(i.tags["Reaper"])

	return &i
}

func (i *Instance) hasTag(tag string) (ok bool) {
	_, ok = i.tags[tag]
	return
}

func (i *Instance) Region() string { return i.region }
func (i *Instance) State() *State  { return i.state }

func (i *Instance) Id() string { return *i.instance.InstanceID }

// Name extracts the "Name" tag
func (i *Instance) Name() string { return i.tags["Name"] }

// Owner extracts the "Owner" tag
func (i *Instance) Owner() string { return i.tags["Owner"] }

// Owned checks if the instance has an Owner tag
func (i *Instance) Owned() (ok bool) { return i.hasTag("Owner") }

// Autoscaled checks if the instance is part of an autoscaling group
func (i *Instance) AutoScaled() (ok bool) { return i.hasTag("aws:autoscaling:groupName") }

func ParseState(state string) (defaultState *State) {

	defaultState = &State{STATE_START, time.Time{}}

	if state == "" {
		return
	}

	s := strings.Split(state, ":")
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
	case "delay":
		stateEnum = STATE_DELAY
	default:
		return
	}

	t, err := time.Parse(time.RFC3339, s[1])
	if err != nil {
		return
	}

	return &State{stateEnum, t}
}

// Filter creates a new list of Instances that match the filter
func (i Instances) Filter(f func(*Instance) bool) (newList Instances) {
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
	in := make(chan *Instance, len(endpoints))

	// fetch all info in parallel
	for region, api := range endpoints {
		wg.Add(1)
		go func(region string, api *ec2.EC2) {
			defer wg.Done()

			resp, err := api.DescribeInstances(nil)
			if err != nil {
				// probably should do something here...
				return
			}

			for _, r := range resp.Reservations {
				for _, instance := range r.Instances {
					in <- NewInstance(region, api, &instance)
				}
			}

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
