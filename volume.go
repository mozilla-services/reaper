package reaper

import (
	"github.com/awslabs/aws-sdk-go/service/ec2"
)

type Volumes []*Volume
type Volume struct {
	AWSResource
	size_gb      int64
	volume_state string
	volume_type  string
	snapshot_id  string
}

func NewVolume(region string, v *ec2.Volume) *Volume {
	vol := Volume{
		AWSResource: AWSResource{
			id:     *v.VolumeID,
			region: region,
			tags:   make(map[string]string),
		},
		size_gb:      *v.Size,
		volume_state: *v.State,
		volume_type:  *v.VolumeType,
		snapshot_id:  *v.SnapshotID,
	}

	for _, tag := range v.Tags {
		vol.tags[*tag.Key] = *tag.Value
	}

	return &vol
}
