package aws

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

// SecurityGroup is a Reapable, Filterable
// embeds AWS API's ec2.SecurityGroup
type SecurityGroup struct {
	Resource
	ec2.SecurityGroup
}

// NewSecurityGroup creates an SecurityGroup from the AWS API's ec2.SecurityGroup
func NewSecurityGroup(region string, sg *ec2.SecurityGroup) *SecurityGroup {
	s := SecurityGroup{
		Resource: Resource{
			ResourceType: "SecurityGroup",
			id:           reapable.ID(*sg.GroupId),
			region:       reapable.Region(region),

			Name: *sg.GroupName,
			Tags: make(map[string]string),
		},
		SecurityGroup: *sg,
	}

	for _, tag := range sg.Tags {
		s.Resource.Tags[*tag.Key] = *tag.Value
	}
	if s.Tagged("aws:cloudformation:stack-name") {
		s.Dependency = true
		s.IsInCloudformation = true
	}
	if s.Tagged(reaperTag) {
		// restore previously tagged state
		s.reaperState = state.NewStateWithTag(s.Resource.Tag(reaperTag))
	} else {
		// initial state
		s.reaperState = state.NewState()
	}

	return &s
}

// Filter is part of the filter.Filterable interface
func (a *SecurityGroup) Filter(filter filters.Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
	case "InCloudformation":
		if b, err := filter.BoolValue(0); err == nil && a.IsInCloudformation == b {
			matched = true
		}
	case "Region":
		for _, region := range filter.Arguments {
			if a.Region() == reapable.Region(region) {
				matched = true
			}
		}
	case "NotRegion":
		// was this resource's region one of those in the NOT list
		regionSpecified := false
		for _, region := range filter.Arguments {
			if a.Region() == reapable.Region(region) {
				regionSpecified = true
			}
		}
		if !regionSpecified {
			matched = true
		}
	case "Tagged":
		if a.Tagged(filter.Arguments[0]) {
			matched = true
		}
	case "NotTagged":
		if !a.Tagged(filter.Arguments[0]) {
			matched = true
		}
	case "TagNotEqual":
		if a.Tag(filter.Arguments[0]) != filter.Arguments[1] {
			matched = true
		}
	case "ReaperState":
		if a.reaperState.State.String() == filter.Arguments[0] {
			matched = true
		}
	case "NotReaperState":
		if a.reaperState.State.String() != filter.Arguments[0] {
			matched = true
		}
	case "Named":
		if a.Name == filter.Arguments[0] {
			matched = true
		}
	case "NotNamed":
		if a.Name != filter.Arguments[0] {
			matched = true
		}
	case "IsDependency":
		if b, err := filter.BoolValue(0); err == nil && a.Dependency == b {
			matched = true
		}
	case "NameContains":
		if strings.Contains(a.Name, filter.Arguments[0]) {
			matched = true
		}
	case "NotNameContains":
		if !strings.Contains(a.Name, filter.Arguments[0]) {
			matched = true
		}
	default:
		log.Error(fmt.Sprintf("No function %s could be found for filtering SecurityGroups.", filter.Function))
	}
	return matched
}

// AWSConsoleURL returns the url that can be used to access the resource on the AWS Console
func (a *SecurityGroup) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/v2/home?region=%s#SecurityGroups:id=%s;view=details",
		a.Region().String(), a.Region().String(), url.QueryEscape(a.ID().String())))
	if err != nil {
		log.Error("Error generating AWSConsoleURL. ", err)
	}
	return url
}

// Terminate is a method of reapable.Terminable, which is embedded in reapable.Reapable
func (a *SecurityGroup) Terminate() (bool, error) {
	log.Info("Terminating SecurityGroup ", a.ReapableDescriptionTiny())
	api := ec2.New(sess, aws.NewConfig().WithRegion(string(a.Region())))

	input := &ec2.DeleteSecurityGroupInput{
		GroupName: aws.String(a.ID().String()),
	}
	_, err := api.DeleteSecurityGroup(input)
	if err != nil {
		log.Error("could not delete SecurityGroup ", a.ReapableDescriptionTiny())
		return false, err
	}
	return false, nil
}

// Stop is a method of reapable.Stoppable, which is embedded in reapable.Reapable
// noop
func (a *SecurityGroup) Stop() (bool, error) {
	return false, nil
}
