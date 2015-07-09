package aws

import (
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/mozilla-services/reaper/reapable"
	"github.com/mozilla-services/reaper/state"
)

type Volume struct {
	AWSResource
	ec2.Volume
}

func NewVolume(region string, v *ec2.Volume) *Volume {
	if v == nil {
		return nil
	}
	vol := Volume{
		AWSResource: AWSResource{
			ID:     reapable.ID(*v.VolumeID),
			Region: reapable.Region(region),
			Tags:   make(map[string]string),
		},
	}

	for _, tag := range v.Tags {
		vol.AWSResource.Tags[*tag.Key] = *tag.Value
	}

	if vol.Tagged(reaperTag) {
		// restore previously tagged state
		vol.reaperState = state.NewStateWithTag(vol.AWSResource.Tag(reaperTag))
	} else {
		// initial state
		vol.reaperState = state.NewStateWithUntilAndState(
			time.Now().Add(config.Notifications.FirstStateDuration.Duration),
			state.FirstState)
	}

	return &vol
}
