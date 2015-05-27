package reaper

import (
	"github.com/awslabs/aws-sdk-go/service/ec2"
	"time"
)

type Snapshots []*Snapshot
type Snapshot struct {
	AWSResource
	size_gb        int64
	snapshot_state string
	volume_id      string
	start_time     time.Time
}

func NewSnapshot(region string, s *ec2.Snapshot) *Snapshot {
	snap := Snapshot{
		AWSResource: AWSResource{
			id:     *s.SnapshotID,
			region: region,
			tags:   make(map[string]string),
		},
		size_gb:        *s.VolumeSize,
		snapshot_state: *s.State,
		volume_id:      *s.VolumeID,
		start_time:     *s.StartTime,
	}

	for _, tag := range s.Tags {
		snap.tags[*tag.Key] = *tag.Value
	}

	return &snap
}
