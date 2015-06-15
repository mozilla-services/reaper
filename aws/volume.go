package aws

import (
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"

	log "github.com/milescrabill/reaper/reaperlog"
)

type Volumes []*Volume
type Volume struct {
	AWSResource
	SizeGB     int64
	VolumeType string
	SnapShotID string
	LaunchTime time.Time
}

func NewVolume(region string, v *ec2.Volume) *Volume {
	vol := Volume{
		AWSResource: AWSResource{
			ID:     *v.VolumeID,
			Region: region,
			Tags:   make(map[string]string),
		},
		SizeGB:     *v.Size,
		VolumeType: *v.VolumeType,
		SnapShotID: *v.SnapshotID,
		LaunchTime: *v.CreateTime,
	}

	for _, tag := range v.Tags {
		vol.Tags[*tag.Key] = *tag.Value
	}

	// TODO: state
	log.Info("Volume state: %s", *v.State)

	return &vol
}
