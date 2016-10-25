package aws

import (
	"fmt"
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
			ResourceType: "Snapshot",
			id:           reapable.ID(*s.SnapshotId),
			region:       reapable.Region(region),
			Tags:         make(map[string]string),
		},
		SizeGB:        *s.VolumeSize,
		SnapshotState: *s.State,
		VolumeID:      reapable.ID(*s.VolumeId),
		LaunchTime:    *s.StartTime,
	}

	for _, tag := range s.Tags {
		snap.Tags[*tag.Key] = *tag.Value
	}

	if snap.Tagged("aws:cloudformation:stack-name") {
		snap.Dependency = true
		snap.IsInCloudformation = true
	}

	return &snap
}

func (a *Snapshot) Filter(filter filters.Filter) bool {
	if isResourceFilter(filter) {
		return a.Resource.Filter(filter)
	}

	// map function names to function calls
	switch filter.Function {
	default:
		log.Error(fmt.Sprintf("No function %s could be found for filtering %s.", filter.Function, a.ResourceType))
	}
	return false
}
