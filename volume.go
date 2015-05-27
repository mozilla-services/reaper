package reaper

import (
	"github.com/awslabs/aws-sdk-go/service/ec2"
	"time"
)

type Volumes []*Volume
type Volume struct {
	AWSResource
	size_gb     int64
	volume_type string
	snapshot_id string
	create_time time.Time
}

func NewVolume(region string, v *ec2.Volume) *Volume {
	vol := Volume{
		AWSResource: AWSResource{
			id:     *v.VolumeID,
			state:  *v.State,
			region: region,
			tags:   make(map[string]string),
		},
		size_gb:     *v.Size,
		volume_type: *v.VolumeType,
		snapshot_id: *v.SnapshotID,
		create_time: *v.CreateTime,
	}

	for _, tag := range v.Tags {
		vol.tags[*tag.Key] = *tag.Value
	}

	return &vol
}

func (v *Volume) LaunchTime() time.Time { return v.create_time }
func (v *Volume) Size() int64           { return v.size_gb }
func (v *Volume) VolumeType() string    { return v.volume_type }
