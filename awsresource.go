package main

import (
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type Terminable interface {
	Terminate() (bool, error)
}

type Stoppable interface {
	Stop() (bool, error)
	ForceStop() (bool, error)
}

type Whitelistable interface {
	Whitelist() (bool, error)
}

//                ,____
//                |---.\
//        ___     |    `
//       / .-\  ./=)
//      |  | |_/\/|
//      ;  |-;| /_|
//     / \_| |/ \ |
//    /      \/\( |
//    |   /  |` ) |
//    /   \ _/    |
//   /--._/  \    |
//   `/|)    |    /
//     /     |   |
//   .'      |   |
//  /         \  |
// (_.-.__.__./  /
// credit: jgs, http://www.chris.com/ascii/index.php?art=creatures/grim%20reapers

type Reapable interface {
	Terminable
	Stoppable
	Whitelistable
	UpdateReaperState(*State) (bool, error)
}

type ResourceState int

const (
	pending ResourceState = iota
	running
	shuttingDown
	terminated
	stopping
	stopped
)

type Filterable interface {
	Filter(Filter) bool
}

func PrintFilters(filters map[string]Filter) string {
	var filterText []string
	for _, filter := range filters {
		filterText = append(filterText, fmt.Sprintf("%s(%s)", filter.Function, strings.Join(filter.Arguments, ", ")))
	}
	// alphabetize and join filters
	sort.Strings(filterText)
	return strings.Join(filterText, ", ")
}

// basic AWS resource, has properties that most/all resources have
type AWSResource struct {
	ID          string
	Name        string
	Region      string
	State       ResourceState
	Description string
	VPCID       string
	OwnerID     string

	Tags map[string]string

	// reaper state
	ReaperState *State
}

func (a *AWSResource) Tagged(tag string) bool {
	_, ok := a.Tags[tag]
	return ok
}

// filter funcs for ResourceState
func (a *AWSResource) Pending() bool      { return a.State == pending }
func (a *AWSResource) Running() bool      { return a.State == running }
func (a *AWSResource) ShuttingDown() bool { return a.State == shuttingDown }
func (a *AWSResource) Terminated() bool   { return a.State == terminated }
func (a *AWSResource) Stopping() bool     { return a.State == stopping }
func (a *AWSResource) Stopped() bool      { return a.State == stopped }

// Tag returns the tag's value or an empty string if it does not exist
func (a *AWSResource) Tag(t string) string { return a.Tags[t] }

func (a *AWSResource) Owned() bool { return a.Tagged("Owner") }

// Owner extracts useful information out of the Owner tag which should
// be parsable by mail.ParseAddress
func (a *AWSResource) Owner() *mail.Address {
	// properly formatted email
	if addr, err := mail.ParseAddress(a.Tag("Owner")); err == nil {
		return addr
	}

	// username -> mozilla.com email
	if addr, err := mail.ParseAddress(fmt.Sprintf("%s@mozilla.com", a.Tag("Owner"))); a.Tagged("Owner") && err == nil {
		return addr
	}

	return nil
}

func (a *AWSResource) ReaperVisible() bool {
	return time.Now().After(a.ReaperState.Until)
}
func (a *AWSResource) ReaperStarted() bool {
	return a.ReaperState.State == STATE_START
}
func (a *AWSResource) ReaperNotified(notifyNum int) bool {
	if notifyNum == 1 {
		return a.ReaperState.State == STATE_NOTIFY1
	} else if notifyNum == 2 {
		return a.ReaperState.State == STATE_NOTIFY2
	} else {
		return false
	}
}

func (a *AWSResource) ReaperIgnored() bool {
	return a.ReaperState.State == STATE_IGNORE
}

func (a *AWSResource) incrementState() bool {
	var newState StateEnum
	var until time.Time

	// did we update state?
	updated := false

	switch a.ReaperState.State {
	case STATE_NOTIFY1:
		updated = true
		newState = STATE_NOTIFY2
		until = time.Now().Add(Conf.Reaper.SecondNotification.Duration)

	case STATE_WHITELIST:
		// keep same state
		newState = STATE_WHITELIST
	case STATE_NOTIFY2:
		updated = true
		newState = STATE_REAPABLE
		until = time.Now().Add(Conf.Reaper.Terminate.Duration)
	case STATE_REAPABLE:
		// keep same state
		newState = STATE_REAPABLE
	default:
		newState = STATE_NOTIFY1
		until = time.Now().Add(Conf.Reaper.FirstNotification.Duration)
	}

	if newState != a.ReaperState.State {
		updated = true
	}

	a.ReaperState = &State{
		State: newState,
		Until: until,
	}

	return updated
}

func (a *AWSResource) Whitelist() (bool, error) {
	return whitelist(a.Region, a.ID)
}

func (a *AWSResource) UpdateReaperState(state *State) (bool, error) {
	return updateReaperState(a.Region, a.ID, state)
}

func whitelist(region, id string) (bool, error) {
	// TODO: fix hardcoded
	whitelist_tag := "REAPER_SPARE_ME"

	api := ec2.New(&aws.Config{Region: region})
	req := &ec2.CreateTagsInput{
		Resources: []*string{aws.String(id)},
		Tags: []*ec2.Tag{
			&ec2.Tag{
				// TODO: not hardcoded
				Key:   aws.String(whitelist_tag),
				Value: aws.String("true"),
			},
		},
	}

	_, err := api.CreateTags(req)

	describereq := &ec2.DescribeTagsInput{
		DryRun: aws.Boolean(false),
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name:   aws.String("resource-id"),
				Values: []*string{aws.String(id)},
			},
			&ec2.Filter{
				Name:   aws.String("key"),
				Values: []*string{aws.String(whitelist_tag)},
			},
		},
	}

	output, err := api.DescribeTags(describereq)

	if *output.Tags[0].Value == whitelist_tag {
		return true, err
	}

	return false, err
}

func updateReaperState(region, id string, newState *State) (bool, error) {
	api := ec2.New(&aws.Config{Region: region})
	createreq := &ec2.CreateTagsInput{
		DryRun:    aws.Boolean(false),
		Resources: []*string{aws.String(id)},
		Tags: []*ec2.Tag{
			&ec2.Tag{
				Key:   aws.String(reaper_tag),
				Value: aws.String(newState.String()),
			},
		},
	}

	_, err := api.CreateTags(createreq)
	if err != nil {
		return false, err
	}

	describereq := &ec2.DescribeTagsInput{
		DryRun: aws.Boolean(false),
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name:   aws.String("resource-id"),
				Values: []*string{aws.String(id)},
			},
			&ec2.Filter{
				Name:   aws.String("key"),
				Values: []*string{aws.String(reaper_tag)},
			},
		},
	}

	output, err := api.DescribeTags(describereq)

	if *output.Tags[0].Value == newState.String() {
		return true, err
	}

	return false, err
}
