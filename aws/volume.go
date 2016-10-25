package aws

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

// Volume is a is a Reapable, Filterable
// embeds AWS API's ec2.Volume
type Volume struct {
	Resource
	ec2.Volume

	AttachedInstanceIDs []string
}

// NewVolume creates an Volume from the AWS API's ec2.Volume
func NewVolume(region string, vol *ec2.Volume) *Volume {
	a := Volume{
		Resource: Resource{
			ResourceType: "Volume",
			region:       reapable.Region(region),
			id:           reapable.ID(*vol.VolumeId),
			Name:         *vol.VolumeId,
			Tags:         make(map[string]string),
		},
		Volume: *vol,
	}

	for _, tag := range vol.Tags {
		a.Resource.Tags[*tag.Key] = *tag.Value
	}

	if a.Tagged("aws:cloudformation:stack-name") {
		a.Dependency = true
		a.IsInCloudformation = true
	}

	for _, attachment := range vol.Attachments {
		if attachment.InstanceId != nil {
			a.AttachedInstanceIDs = append(a.AttachedInstanceIDs, *attachment.InstanceId)
		}
	}

	if a.Tagged(reaperTag) {
		// restore previously tagged state
		a.reaperState = state.NewStateWithTag(a.Tag(reaperTag))
	} else {
		// initial state
		a.reaperState = state.NewStateWithUntilAndState(
			time.Now().Add(config.Notifications.FirstStateDuration.Duration),
			state.FirstState)
	}

	return &a
}

func (a *Volume) sizeGreaterThanOrEqualTo(size int64) bool {
	if a.Size != nil {
		return *a.Size >= size
	}
	return false
}

func (a *Volume) sizeLessThanOrEqualTo(size int64) bool {
	if a.Size != nil {
		return *a.Size <= size
	}
	return false
}

func (a *Volume) sizeEqualTo(size int64) bool {
	if a.Size != nil {
		return *a.Size == size
	}
	return false
}

func (a *Volume) sizeLessThan(size int64) bool {
	if a.Size != nil {
		return *a.Size < size
	}
	return false
}

func (a *Volume) sizeGreaterThan(size int64) bool {
	if a.Size != nil {
		return *a.Size > size
	}
	return false
}

// Filter is part of the filter.Filterable interface
func (a *Volume) Filter(filter filters.Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
	case "SizeGreaterThan":
		if i, err := filter.Int64Value(0); err == nil && a.sizeGreaterThan(i) {
			matched = true
		}
	case "SizeLessThan":
		if i, err := filter.Int64Value(0); err == nil && a.sizeLessThan(i) {
			matched = true
		}
	case "SizeEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.sizeEqualTo(i) {
			matched = true
		}
	case "SizeLessThanOrEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.sizeLessThanOrEqualTo(i) {
			matched = true
		}
	case "SizeGreaterThanOrEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.sizeGreaterThanOrEqualTo(i) {
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
	case "Region":
		for region := range filter.Arguments {
			if a.Region() == reapable.Region(region) {
				matched = true
			}
		}
	case "NotRegion":
		for region := range filter.Arguments {
			if a.Region() == reapable.Region(region) {
				matched = false
			}
		}
	case "CreatedInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && a.CreateTime != nil && time.Since(*a.CreateTime) < d {
			matched = true
		}
	case "CreatedNotInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && a.CreateTime != nil && time.Since(*a.CreateTime) > d {
			matched = true
		}
	case "InCloudformation":
		if b, err := filter.BoolValue(0); err == nil && a.IsInCloudformation == b {
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
	case "State":
		// one of:
		// creating
		// available
		// in-use
		// deleting
		// deleted
		// error

		if a.State != nil && *a.State == filter.Arguments[0] {
			matched = true
		}
	case "AttachmentState":
		// one of:
		// attaching
		// attached
		// detaching
		// detached

		// I _think_ that the size of Attachments is only 0 or 1
		if len(a.Attachments) > 0 && *a.Attachments[0].State == filter.Arguments[0] {
			matched = true
		} else if len(a.Attachments) == 0 && "detached" == filter.Arguments[0] {
			matched = true
		}
	default:
		log.Error(fmt.Sprintf("No function %s could be found for filtering Volumes.", filter.Function))
	}
	return matched
}

// AWSConsoleURL returns the url that can be used to access the resource on the AWS Console
func (a *Volume) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/v2/home?region=%s#Volumes:volumeId=%s",
		a.Region().String(), a.Region().String(), url.QueryEscape(a.ID().String())))
	if err != nil {
		log.Error("Error generating AWSConsoleURL. ", err)
	}
	return url
}

// Terminate is a method of reapable.Terminable, which is embedded in reapable.Reapable
func (a *Volume) Terminate() (bool, error) {
	log.Info("Terminating Volume ", a.ReapableDescriptionTiny())
	api := ec2.New(sess, aws.NewConfig().WithRegion(string(a.Region())))
	input := &ec2.DeleteVolumeInput{
		VolumeId: aws.String(a.ID().String()),
	}
	_, err := api.DeleteVolume(input)
	if err != nil {
		log.Error("could not delete Volume ", a.ReapableDescriptionTiny())
		return false, err
	}
	return true, nil
}

// Stop is a method of reapable.Stoppable, which is embedded in reapable.Reapable
// noop
func (a *Volume) Stop() (bool, error) {
	// use existing min size
	return false, nil
}
