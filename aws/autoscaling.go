package aws

import (
	"fmt"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"

	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

// AutoScalingGroup is a Reapable, Filterable
// embeds AWS API's autoscaling.Group
type AutoScalingGroup struct {
	Resource
	autoscaling.Group

	// autoscaling.Instance exposes minimal info
	Instances []reapable.ID
}

// NewAutoScalingGroup creates an AutoScalingGroup from the AWS API's autoscaling.Group
func NewAutoScalingGroup(region string, asg *autoscaling.Group) *AutoScalingGroup {
	a := AutoScalingGroup{
		Resource: Resource{
			ResourceType: "AutoScalingGroup",
			region:       reapable.Region(region),
			id:           reapable.ID(*asg.AutoScalingGroupName),
			Name:         *asg.AutoScalingGroupName,
			Tags:         make(map[string]string),
		},
		Group: *asg,
	}

	for _, instance := range asg.Instances {
		a.Instances = append(a.Instances, reapable.ID(*instance.InstanceId))
	}

	for _, tag := range asg.Tags {
		a.Resource.Tags[*tag.Key] = *tag.Value
	}

	if a.Tagged("aws:cloudformation:stack-name") {
		a.Dependency = true
		a.IsInCloudformation = true
	}

	if a.Tagged(reaperTag) {
		// restore previously tagged state
		a.reaperState = state.NewStateWithTag(a.Tag(reaperTag))
	} else {
		// initial state
		a.reaperState = state.NewState()
	}

	return &a
}

func (a *AutoScalingGroup) sizeGreaterThanOrEqualTo(size int64) bool {
	if a.DesiredCapacity != nil {
		return *a.DesiredCapacity >= size
	}
	return false
}

func (a *AutoScalingGroup) sizeLessThanOrEqualTo(size int64) bool {
	if a.DesiredCapacity != nil {
		return *a.DesiredCapacity <= size
	}
	return false
}

func (a *AutoScalingGroup) sizeEqualTo(size int64) bool {
	if a.DesiredCapacity != nil {
		return *a.DesiredCapacity == size
	}
	return false
}

func (a *AutoScalingGroup) sizeLessThan(size int64) bool {
	if a.DesiredCapacity != nil {
		return *a.DesiredCapacity < size
	}
	return false
}

func (a *AutoScalingGroup) sizeGreaterThan(size int64) bool {
	if a.DesiredCapacity != nil {
		return *a.DesiredCapacity > size
	}
	return false
}

// Save is part of reapable.Saveable, which embedded in reapable.Reapable
func (a *AutoScalingGroup) Save(s *state.State) (bool, error) {
	return tagAutoScalingGroup(a.Region(), a.ID(), reaperTag, a.reaperState.String())
}

// Unsave is part of reapable.Saveable, which embedded in reapable.Reapable
func (a *AutoScalingGroup) Unsave() (bool, error) {
	log.Info("Unsaving %s", a.ReapableDescriptionTiny())
	return untagAutoScalingGroup(a.Region(), a.ID(), reaperTag)
}

func untagAutoScalingGroup(region reapable.Region, id reapable.ID, key string) (bool, error) {
	api := autoscaling.New(session.New(&aws.Config{Region: aws.String(string(region))}))
	deletereq := &autoscaling.DeleteTagsInput{
		Tags: []*autoscaling.Tag{
			&autoscaling.Tag{
				ResourceId:   aws.String(string(id)),
				ResourceType: aws.String("auto-scaling-group"),
				Key:          aws.String(key),
			},
		},
	}

	_, err := api.DeleteTags(deletereq)
	if err != nil {
		return false, err
	}

	return true, nil
}

func tagAutoScalingGroup(region reapable.Region, id reapable.ID, key, value string) (bool, error) {
	log.Info("Tagging AutoScalingGroup %s in %s with %s:%s", region.String(), id.String(), key, value)
	api := autoscaling.New(session.New(&aws.Config{Region: aws.String(region.String())}))
	createreq := &autoscaling.CreateOrUpdateTagsInput{
		Tags: []*autoscaling.Tag{
			&autoscaling.Tag{
				ResourceId:        aws.String(string(id)),
				ResourceType:      aws.String("auto-scaling-group"),
				PropagateAtLaunch: aws.Bool(false),
				Key:               aws.String(key),
				Value:             aws.String(value),
			},
		},
	}

	_, err := api.CreateOrUpdateTags(createreq)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Filter is part of the filter.Filterable interface
func (a *AutoScalingGroup) Filter(filter filters.Filter) bool {
	if isResourceFilter(filter) {
		return a.Resource.Filter(filter)
	}
	// map function names to function calls
	switch filter.Function {
	case "SizeGreaterThan":
		if i, err := filter.Int64Value(0); err == nil && a.sizeGreaterThan(i) {
			return true
		}
	case "SizeLessThan":
		if i, err := filter.Int64Value(0); err == nil && a.sizeLessThan(i) {
			return true
		}
	case "SizeEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.sizeEqualTo(i) {
			return true
		}
	case "SizeLessThanOrEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.sizeLessThanOrEqualTo(i) {
			return true
		}
	case "SizeGreaterThanOrEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.sizeGreaterThanOrEqualTo(i) {
			return true
		}
	case "CreatedTimeInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && a.CreatedTime != nil && time.Since(*a.CreatedTime) < d {
			return true
		}
	case "CreatedTimeNotInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && a.CreatedTime != nil && time.Since(*a.CreatedTime) > d {
			return true
		}
	default:
		log.Error(fmt.Sprintf("No function %s could be found for filtering %s.", filter.Function, a.ResourceType))
	}
	return false
}

// AWSConsoleURL returns the url that can be used to access the resource on the AWS Console
func (a *AutoScalingGroup) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/autoscaling/home?region=%s#AutoScalingGroups:id=%s;view=details",
		a.Region().String(), a.Region().String(), url.QueryEscape(a.ID().String())))
	if err != nil {
		log.Error("Error generating AWSConsoleURL. ", err)
	}
	return url
}

func (a *AutoScalingGroup) scaleToSize(size int64, minSize int64) (bool, error) {
	log.Info("Scaling AutoScalingGroup %s to size %d.", a.ReapableDescriptionTiny(), size)
	as := autoscaling.New(session.New(&aws.Config{Region: aws.String(a.Region().String())}))
	input := &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(a.ID().String()),
		DesiredCapacity:      &size,
		MinSize:              &minSize,
	}

	_, err := as.UpdateAutoScalingGroup(input)
	if err != nil {
		log.Error("could not update AutoScalingGroup ", a.ReapableDescriptionTiny())
		return false, err
	}
	return true, nil
}

// Terminate is a method of reapable.Terminable, which is embedded in reapable.Reapable
func (a *AutoScalingGroup) Terminate() (bool, error) {
	log.Info("Terminating AutoScalingGroup %s", a.ReapableDescriptionTiny())
	as := autoscaling.New(session.New(&aws.Config{Region: aws.String(a.Region().String())}))
	input := &autoscaling.DeleteAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(a.ID().String()),
	}
	_, err := as.DeleteAutoScalingGroup(input)
	if err != nil {
		log.Error("could not delete AutoScalingGroup ", a.ReapableDescriptionTiny())
		return false, err
	}
	return true, nil
}

// Whitelist is a method of reapable.Whitelistable, which is embedded in reapable.Reapable
func (a *AutoScalingGroup) Whitelist() (bool, error) {
	log.Info("Whitelisting AutoScalingGroup %s", a.ReapableDescriptionTiny())
	api := autoscaling.New(session.New(&aws.Config{Region: aws.String(a.Region().String())}))
	createreq := &autoscaling.CreateOrUpdateTagsInput{
		Tags: []*autoscaling.Tag{
			&autoscaling.Tag{
				ResourceId:        aws.String(a.ID().String()),
				ResourceType:      aws.String("auto-scaling-group"),
				PropagateAtLaunch: aws.Bool(false),
				Key:               aws.String(config.WhitelistTag),
				Value:             aws.String("true"),
			},
		},
	}
	_, err := api.CreateOrUpdateTags(createreq)
	return err == nil, err
}

// Stop is a method of reapable.Stoppable, which is embedded in reapable.Reapable
// Stop scales ASGs to 0
func (a *AutoScalingGroup) Stop() (bool, error) {
	// use existing min size
	return a.scaleToSize(0, 0)
}
