package aws

import (
	"bytes"
	"fmt"
	htmlTemplate "html/template"
	"net/mail"
	textTemplate "text/template"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

// Resource has properties shared by all AWS resources
type Resource struct {
	id     reapable.ID
	region reapable.Region

	Name               string
	Dependency         bool
	IsInCloudformation bool

	Tags map[string]string

	// reaper state
	reaperState *state.State

	// filters for MatchedFilters
	matchedFilterGroups map[string]filters.FilterGroup
}

// ID is a method of reapable
func (a *Resource) ID() reapable.ID {
	return a.id
}

// Region is a method of reapable
func (a *Resource) Region() reapable.Region {
	return a.region
}

// Tagged returns whether the Resource is tagged with that key
func (a *Resource) Tagged(tag string) bool {
	_, ok := a.Tags[tag]
	return ok
}

// Tag returns the tag's value or an empty string if it does not exist
func (a *Resource) Tag(t string) string {
	return a.Tags[t]
}

// Owned returns whether the Resource has a clear owner
// if a DefaultOwner is set, there is always an owner
func (a *Resource) Owned() bool {
	// if the resource has an owner tag or a default owner is specified
	return a.Tagged("Owner") || config.DefaultOwner != ""
}

// ReaperState is a method of reapable.Saveable, which is embedded in reapable.Reapable
func (a *Resource) ReaperState() *state.State {
	return a.reaperState
}

// SetReaperState sets the ReaperState for a Resource
func (a *Resource) SetReaperState(newState *state.State) {
	a.reaperState = newState
}

func (a *Resource) SetUpdated(b bool) {
	a.reaperState.Updated = b
}

// Owner extracts useful information out of the Owner tag which should
// be parsable by mail.ParseAddress
func (a *Resource) Owner() *mail.Address {
	// properly formatted email
	if addr, err := mail.ParseAddress(a.Tag("Owner")); err == nil {
		return addr
	}

	// username -> default email host email address
	if addr, err := mail.ParseAddress(fmt.Sprintf("%s@%s", a.Tag("Owner"), config.DefaultEmailHost)); a.Tagged("Owner") && config.DefaultEmailHost != "" && err == nil {
		return addr
	}

	// default owner is specified
	var addr *mail.Address
	var err error
	if config.DefaultOwner != "" {
		addr, err = mail.ParseAddress(config.DefaultOwner)
	}
	if err != nil {
		log.Error("DefaultOwner not properly set: %s", err.Error())
	}
	if addr == nil {
		log.Error("DefaultOwner not properly set.")
		return nil
	}
	return addr
}

// IncrementState updates the ReaperState of a Resource
// returns a boolean of whether it was updated
func (a *Resource) IncrementState() (updated bool) {
	var newState state.StateEnum
	until := time.Now()
	switch a.reaperState.State {
	default:
		fallthrough
	case state.InitialState:
		// set state to the FirstState
		newState = state.FirstState
		until = until.Add(config.Notifications.FirstStateDuration.Duration)
	case state.FirstState:
		// go to SecondState at the end of FirstState
		newState = state.SecondState
		until = until.Add(config.Notifications.SecondStateDuration.Duration)
	case state.SecondState:
		// go to ThirdState at the end of SecondState
		newState = state.ThirdState
		until = until.Add(config.Notifications.ThirdStateDuration.Duration)
	case state.ThirdState:
		// go to FinalState at the end of ThirdState
		newState = state.FinalState
	case state.FinalState:
		// keep same state
		newState = state.FinalState
	case state.IgnoreState:
		// keep same state
		newState = state.IgnoreState
	}

	if newState != a.reaperState.State {
		updated = true
		a.reaperState = state.NewStateWithUntilAndState(until, newState)
		log.Info("Updating state for %s. New state: %s.", a.ReapableDescriptionTiny(), newState.String())
	}

	return updated
}

// AddFilterGroup is a method of filter.Filterable
func (a *Resource) AddFilterGroup(name string, fs filters.FilterGroup) {
	if a.matchedFilterGroups == nil {
		a.matchedFilterGroups = make(map[string]filters.FilterGroup)
	}
	a.matchedFilterGroups[name] = fs
}

// MatchedFiltersString returns a formatted string with the filters the Resource matched
func (a *Resource) MatchedFiltersString() string {
	return filters.FormatFilterGroupsText(a.matchedFilterGroups)
}

type templater interface {
	getTemplateData() (interface{}, error)
}

func reapableEventHTML(a templater, text string) (*bytes.Buffer, error) {
	t := htmlTemplate.Must(htmlTemplate.New("reapable").Parse(text))
	buf := bytes.NewBuffer(nil)

	data, err := a.getTemplateData()
	if err != nil {
		return nil, err
	}
	err = t.Execute(buf, data)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func reapableEventText(a templater, text string) (*bytes.Buffer, error) {
	t := textTemplate.Must(textTemplate.New("reapable").Parse(text))
	buf := bytes.NewBuffer(nil)

	data, err := a.getTemplateData()
	if err != nil {
		return nil, err
	}
	err = t.Execute(buf, data)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// ReapableDescription is a method of reapable.Reapable
func (a *Resource) ReapableDescription() string {
	return fmt.Sprintf("%s matched %s", a.ReapableDescriptionShort(), a.MatchedFiltersString())
}

// ReapableDescriptionShort is a method of reapable.Reapable
func (a *Resource) ReapableDescriptionShort() string {
	ownerString := ""
	if owner := a.Owner(); owner != nil {
		ownerString = fmt.Sprintf(" (owned by %s)", owner)
	}
	nameString := ""
	if name := a.Tag("Name"); name != "" {
		nameString = fmt.Sprintf(" \"%s\"", name)
	}
	return fmt.Sprintf("'%s'%s%s in %s with state: %s", a.ID(), nameString, ownerString, a.Region(), a.ReaperState().String())
}

// ReapableDescriptionTiny is a method of reapable.Reapable
func (a *Resource) ReapableDescriptionTiny() string {
	return fmt.Sprintf("'%s' in %s", a.ID(), a.Region())
}

// Whitelist is a method of reapable.Whitelistable, which is embedded in reapable.Reapable
func (a *Resource) Whitelist() (bool, error) {
	return tag(a.Region().String(), a.ID().String(), config.WhitelistTag, "true")
}

// Save is a method of reapable.Saveable, which is embedded in reapable.Reapable
// Save tags a Resource's reaperTag
func (a *Resource) Save(reaperState *state.State) (bool, error) {
	log.Info("Saving %s", a.ReapableDescriptionTiny())
	return tag(a.Region().String(), a.ID().String(), reaperTag, reaperState.String())
}

// Unsave is a method of reapable.Saveable, which is embedded in reapable.Reapable
// Unsave untags a Resource's reaperTag
func (a *Resource) Unsave() (bool, error) {
	log.Info("Unsaving %s", a.ReapableDescriptionTiny())
	return untag(a.Region().String(), a.ID().String(), reaperTag)
}

func untag(region, id, key string) (bool, error) {
	api := ec2.New(sess, aws.NewConfig().WithRegion(string(region)))
	delreq := &ec2.DeleteTagsInput{
		DryRun:    aws.Bool(false),
		Resources: []*string{aws.String(id)},
		Tags: []*ec2.Tag{
			&ec2.Tag{
				Key: aws.String(key),
			},
		},
	}
	_, err := api.DeleteTags(delreq)
	if err != nil {
		return false, err
	}
	return true, err
}

func tag(region, id, key, value string) (bool, error) {
	api := ec2.New(sess, aws.NewConfig().WithRegion(string(region)))
	createreq := &ec2.CreateTagsInput{
		DryRun:    aws.Bool(false),
		Resources: []*string{aws.String(id)},
		Tags: []*ec2.Tag{
			&ec2.Tag{
				Key:   aws.String(key),
				Value: aws.String(value),
			},
		},
	}

	_, err := api.CreateTags(createreq)
	if err != nil {
		return false, err
	}

	describereq := &ec2.DescribeTagsInput{
		DryRun: aws.Bool(false),
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name:   aws.String("resource-id"),
				Values: []*string{aws.String(id)},
			},
			&ec2.Filter{
				Name:   aws.String("key"),
				Values: []*string{aws.String(key)},
			},
		},
	}

	output, err := api.DescribeTags(describereq)

	if *output.Tags[0].Value == value {
		return true, err
	}

	return false, err
}
