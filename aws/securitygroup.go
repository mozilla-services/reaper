package aws

import (
	"fmt"
	"net/url"

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
	if isResourceFilter(filter) {
		return a.Resource.Filter(filter)
	}

	// map function names to function calls
	switch filter.Function {
	default:
		log.Error(fmt.Sprintf("No function %s could be found for filtering %s.", filter.Function, a.ResourceType))
	}
	return false
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
