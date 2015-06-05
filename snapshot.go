package main

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"time"
)

type Snapshot struct {
	AWSResource
	SizeGB        int64
	SnapshotState string
	VolumeId      string
	LaunchTime    time.Time
}

func NewSnapshot(region string, s *ec2.Snapshot) *Snapshot {
	snap := Snapshot{
		AWSResource: AWSResource{
			Id:     *s.SnapshotID,
			Region: region,
			Tags:   make(map[string]string),
		},
		SizeGB:        *s.VolumeSize,
		SnapshotState: *s.State,
		VolumeId:      *s.VolumeID,
		LaunchTime:    *s.StartTime,
	}

	for _, tag := range s.Tags {
		snap.Tags[*tag.Key] = *tag.Value
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
