package events

import (
	"fmt"

	log "github.com/mozilla-services/reaper/reaperlog"
)

// ReaperEventConfig is the configuration for a ReaperEvent
type ReaperEventConfig struct {
	eventReporterConfig

	Mode string
}

// ReaperEvent implements ReapableEventReporter, terminates resources
type ReaperEvent struct {
	Config *ReaperEventConfig
}

// SetDryRun is a method of ReapableEventReporter
func (e *ReaperEvent) SetDryRun(b bool) {
	e.Config.DryRun = b
}

// NewReaperEvent returns a new instance of ReaperEvent
func NewReaperEvent(c *ReaperEventConfig) *ReaperEvent {
	c.Name = "ReaperEvent"
	return &ReaperEvent{c}
}

// NewReapableEvent is a method of ReapableEventReporter
func (e *ReaperEvent) NewReapableEvent(r Reapable, tags []string) error {
	if e.Config.shouldTriggerFor(r) {
		if log.Extras() {
			log.Error("Triggering ReaperEvent for %s", r.ReaperState().String())
		}
		var err error
		switch e.Config.Mode {
		case "Stop":
			_, err = r.Stop()
		case "Terminate":
			_, err = r.Terminate()
		default:
			log.Error(fmt.Sprintf("Invalid %s Mode %s", e.Config.Name, e.Config.Mode))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// NewBatchReapableEvent is a method of ReapableEventReporter
func (e *ReaperEvent) NewBatchReapableEvent(rs []Reapable, tags []string) error {
	for _, r := range rs {
		err := e.NewReapableEvent(r, tags)
		if err != nil {
			return err
		}
	}
	return nil
}
