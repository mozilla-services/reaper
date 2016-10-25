package aws

import (
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"

	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

// Cloudformation is a Reapable, Filterable
// embeds AWS API's cloudformation.Stack
type Cloudformation struct {
	Resource
	cloudformation.Stack
	Resources []cloudformation.StackResource
	// locks because of CloudformationResources access
	sync.RWMutex
}

// NewCloudformation creates a new Cloudformation from the AWS API's cloudformation.Stack
func NewCloudformation(region string, stack *cloudformation.Stack) *Cloudformation {
	a := Cloudformation{
		Resource: Resource{
			ResourceType: "Cloudformation",
			region:       reapable.Region(region),
			id:           reapable.ID(*stack.StackId),
			Name:         *stack.StackName,
			Tags:         make(map[string]string),
			reaperState:  state.NewStateWithUntil(time.Now().Add(config.Notifications.FirstStateDuration.Duration)),
		},
		Stack: *stack,
	}

	// because getting resources is rate limited...
	go func() {
		a.Lock()
		for resource := range cloudformationResources(a.Region().String(), a.ID().String()) {
			a.Resources = append(a.Resources, *resource)
		}
		a.Unlock()
	}()

	for _, tag := range stack.Tags {
		a.Resource.Tags[*tag.Key] = *tag.Value
	}

	if a.Tagged(reaperTag) {
		// restore previously tagged state
		a.reaperState = state.NewStateWithTag(a.Resource.Tag(reaperTag))
	} else {
		// initial state
		a.reaperState = state.NewState()
	}

	return &a
}

// Save is part of reapable.Saveable, which embedded in reapable.Reapable
// no op because we cannot tag cloudformations without updating the stack
func (a *Cloudformation) Save(s *state.State) (bool, error) {
	return false, nil
}

// Unsave is part of reapable.Saveable, which embedded in reapable.Reapable
// no op because we cannot tag cloudformations without updating the stack
func (a *Cloudformation) Unsave() (bool, error) {
	log.Info("Unsaving %s", a.ReapableDescriptionTiny())
	return false, nil
}

// Filter is part of the filter.Filterable interface
func (a *Cloudformation) Filter(filter filters.Filter) bool {
	if isResourceFilter(filter) {
		return a.Resource.Filter(filter)
	}

	// map function names to function calls
	switch filter.Function {
	case "Status":
		if a.StackStatus != nil && *a.StackStatus == filter.Arguments[0] {
			// one of:
			// CREATE_COMPLETE
			// CREATE_IN_PROGRESS
			// CREATE_FAILED
			// DELETE_COMPLETE
			// DELETE_FAILED
			// DELETE_IN_PROGRESS
			// ROLLBACK_COMPLETE
			// ROLLBACK_FAILED
			// ROLLBACK_IN_PROGRESS
			// UPDATE_COMPLETE
			// UPDATE_COMPLETE_CLEANUP_IN_PROGRESS
			// UPDATE_IN_PROGRESS
			// UPDATE_ROLLBACK_COMPLETE
			// UPDATE_ROLLBACK_COMPLETE_CLEANUP_IN_PROGRESS
			// UPDATE_ROLLBACK_FAILED
			// UPDATE_ROLLBACK_IN_PROGRESS
			return true
		}
	case "NotStatus":
		if a.StackStatus != nil && *a.StackStatus != filter.Arguments[0] {
			return true
		}
	case "CreatedTimeInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && a.CreationTime != nil && time.Since(*a.CreationTime) < d {
			return true
		}
	case "CreatedTimeNotInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && a.CreationTime != nil && time.Since(*a.CreationTime) > d {
			return true
		}
	default:
		log.Error(fmt.Sprintf("No function %s could be found for filtering %s.", filter.Function, a.ResourceType))
	}
	return false
}

// AWSConsoleURL returns the url that can be used to access the resource on the AWS Console
func (a *Cloudformation) AWSConsoleURL() *url.URL {
	url, err := url.Parse("https://console.aws.amazon.com/cloudformation/home")
	// setting RawQuery because QueryEscape messes with the "/"s in the url
	url.RawQuery = fmt.Sprintf("region=%s#/stacks?filter=active&tab=overview&stackId=%s", a.Region().String(), a.ID().String())
	if err != nil {
		log.Error("Error generating AWSConsoleURL. ", err)
	}
	return url
}

// Terminate is a method of reapable.Terminable, which is embedded in reapable.Reapable
func (a *Cloudformation) Terminate() (bool, error) {
	log.Info("Terminating Cloudformation %s", a.ReapableDescriptionTiny())
	as := cloudformation.New(sess, aws.NewConfig().WithRegion(a.Region().String()))

	input := &cloudformation.DeleteStackInput{
		StackName: aws.String(a.ID().String()),
	}
	_, err := as.DeleteStack(input)
	if err != nil {
		log.Error(fmt.Sprintf("could not delete %s %s", a.ResourceType, a.ReapableDescriptionTiny()))
		return false, err
	}
	return false, nil
}

// Whitelist is a method of reapable.Whitelistable, which is embedded in reapable.Reapable
// no op because we cannot tag cloudformations without updating the stack
func (a *Cloudformation) Whitelist() (bool, error) {
	return false, nil
}

// Stop is a method of reapable.Stoppable, which is embedded in reapable.Reapable
// no op because there is no concept of stopping a cloudformation
func (a *Cloudformation) Stop() (bool, error) {
	return false, nil
}
