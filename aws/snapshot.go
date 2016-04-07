package aws

import (
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
)

type Snapshot struct {
	Resource
	SizeGB        int64
	SnapshotState string
	VolumeID      reapable.ID
	LaunchTime    time.Time
}

func NewSnapshot(region string, s *ec2.Snapshot) *Snapshot {
	snap := Snapshot{
		Resource: Resource{
			ID:     reapable.ID(*s.SnapshotID),
			Region: reapable.Region(region),
			Tags:   make(map[string]string),
		},
		SizeGB:        *s.VolumeSize,
		SnapshotState: *s.State,
		VolumeID:      reapable.ID(*s.VolumeID),
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
		log.Error("No function %s could be found for filtering Snapshots.", filter.Function)
	}
	return matched
}
