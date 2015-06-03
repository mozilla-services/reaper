package main

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"time"
)

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

func (s *Snapshot) Filter(filter Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
	default:
		Log.Error("No function %s could be found for filtering Snapshots.", filter.Function)
	}
	return matched
}
