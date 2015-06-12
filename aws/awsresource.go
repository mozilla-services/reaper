package aws

import (
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/mostlygeek/reaper/filters"
	"github.com/mostlygeek/reaper/state"
)

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
	Filter(filters.Filter) bool
}

func PrintFilters(filters map[string]filters.Filter) string {
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
	ID            string
	Name          string
	Region        string
	ResourceState ResourceState
	Description   string
	VPCID         string
	OwnerID       string

	Tags map[string]string

	// reaper state
	reaperState *state.State
}

func (a *AWSResource) Tagged(tag string) bool {
	_, ok := a.Tags[tag]
	return ok
}

// filter funcs for ResourceState
func (a *AWSResource) Pending() bool      { return a.ResourceState == pending }
func (a *AWSResource) Running() bool      { return a.ResourceState == running }
func (a *AWSResource) ShuttingDown() bool { return a.ResourceState == shuttingDown }
func (a *AWSResource) Terminated() bool   { return a.ResourceState == terminated }
func (a *AWSResource) Stopping() bool     { return a.ResourceState == stopping }
func (a *AWSResource) Stopped() bool      { return a.ResourceState == stopped }

// Tag returns the tag's value or an empty string if it does not exist
func (a *AWSResource) Tag(t string) string { return a.Tags[t] }

func (a *AWSResource) Owned() bool { return a.Tagged("Owner") }

func (a *AWSResource) ReaperState() *state.State {
	return a.reaperState
}

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
	return time.Now().After(a.reaperState.Until)
}
func (a *AWSResource) ReaperStarted() bool {
	return a.reaperState.State == state.STATE_START
}
func (a *AWSResource) ReaperNotified(notifyNum int) bool {
	if notifyNum == 1 {
		return a.reaperState.State == state.STATE_NOTIFY1
	} else if notifyNum == 2 {
		return a.reaperState.State == state.STATE_NOTIFY2
	} else {
		return false
	}
}

func (a *AWSResource) IncrementState() bool {
	var newState state.StateEnum
	until := time.Now()

	// did we update state?
	updated := false

	switch a.reaperState.State {
	case state.STATE_NOTIFY1:
		updated = true
		newState = state.STATE_NOTIFY2
		until = until.Add(config.Notifications.SecondNotification.Duration)

	case state.STATE_WHITELIST:
		// keep same state
		newState = state.STATE_WHITELIST
	case state.STATE_NOTIFY2:
		newState = state.STATE_REAPABLE
		until = until.Add(config.Notifications.Terminate.Duration)
	case state.STATE_REAPABLE:
		// keep same state
		newState = state.STATE_REAPABLE
	case state.STATE_START:
		newState = state.STATE_NOTIFY1
		until = until.Add(config.Notifications.FirstNotification.Duration)
	default:
		log.Notice("Unrecognized state %s ", a.reaperState.State)
		newState = a.reaperState.State
	}

	if newState != a.reaperState.State {
		updated = true
		log.Debug("Updating state on %s in region %s. New state: %s.", a.ID, a.Region, newState.String())
	}

	a.reaperState = state.NewStateWithUntilAndState(until, newState)

	return updated
}

func (a *AWSResource) ReapableDescription() string {
	ownerString := ""
	if owner := a.Owner(); owner != nil {
		ownerString = fmt.Sprintf(" (owned by %s)", owner)
	}
	return fmt.Sprintf("'%s' in %s%s", a.ID, a.Region, ownerString)
}

func (a *AWSResource) Whitelist() (bool, error) {
	return Whitelist(a.Region, a.ID)
}

// methods for reapable interface:
func (a *AWSResource) Save(s *state.State) (bool, error) {
	return TagReaperState(a.Region, a.ID, s)
}

func (a *AWSResource) Unsave() (bool, error) {
	return UntagReaperState(a.Region, a.ID)
}

func Whitelist(region, id string) (bool, error) {
	whitelist_tag := config.WhitelistTag

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
		log.Info("Whitelist successful.")
		return true, err
	}

	return false, err
}

func UntagReaperState(region, id string) (bool, error) {
	api := ec2.New(&aws.Config{Region: region})
	delreq := &ec2.DeleteTagsInput{
		DryRun:    aws.Boolean(false),
		Resources: []*string{aws.String(id)},
		Tags: []*ec2.Tag{
			&ec2.Tag{
				Key: aws.String(reaperTag),
			},
		},
	}
	_, err := api.DeleteTags(delreq)
	if err != nil {
		return false, err
	}
	return true, err
}

func TagReaperState(region, id string, newState *state.State) (bool, error) {
	api := ec2.New(&aws.Config{Region: region})
	createreq := &ec2.CreateTagsInput{
		DryRun:    aws.Boolean(false),
		Resources: []*string{aws.String(id)},
		Tags: []*ec2.Tag{
			&ec2.Tag{
				Key:   aws.String(reaperTag),
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
				Values: []*string{aws.String(reaperTag)},
			},
		},
	}

	output, err := api.DescribeTags(describereq)

	if *output.Tags[0].Value == newState.String() {
		return true, err
	}

	return false, err
}
