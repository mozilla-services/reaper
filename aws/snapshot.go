package aws

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/mostlygeek/reaper/filters"
)

type Snapshot struct {
	AWSResource
	SizeGB        int64
	SnapshotState string
	VolumeID      string
	LaunchTime    time.Time
}

func NewSnapshot(region string, s *ec2.Snapshot) *Snapshot {
	snap := Snapshot{
		AWSResource: AWSResource{
			ID:     *s.SnapshotID,
			Region: region,
			Tags:   make(map[string]string),
		},
		SizeGB:        *s.VolumeSize,
		SnapshotState: *s.State,
		VolumeID:      *s.VolumeID,
		LaunchTime:    *s.StartTime,
	}

	for _, tag := range s.Tags {
		snap.Tags[*tag.Key] = *tag.Value
	}

	return &snap
}

func (s *Snapshot) Filter(filter filters.Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
	default:
		log.Error(fmt.Sprintf("No function %s could be found for filtering Snapshots.", filter.Function))
	}
	return matched
}
