package events

import (
	"fmt"

	log "github.com/mozilla-services/reaper/reaperlog"
)

// ReaperEventConfig is the configuration for a ReaperEvent
type ReaperEventConfig struct {
	*EventReporterConfig

	Mode string
}

// ReaperEvent implements EventReporter, terminates resources
type ReaperEvent struct {
	Config *ReaperEventConfig
}

// setDryRun is a method of EventReporter
func (e *ReaperEvent) setDryRun(b bool) {
	e.Config.DryRun = b
}

// NewReaperEvent returns a new instance of ReaperEvent
func NewReaperEvent(c *ReaperEventConfig) *ReaperEvent {
	c.Name = "ReaperEvent"
	return &ReaperEvent{c}
}

// newReapableEvent is a method of EventReporter
func (e *ReaperEvent) newReapableEvent(r Reapable, tags []string) error {
	if e.Config.shouldTriggerFor(r) {
		var err error
		switch e.Config.Mode {
		case "Stop":
			_, err = r.Stop()
			if log.Extras() {
				log.Info("ReaperEvent: Stopping ", r.ReapableDescriptionTiny())
			}
		case "Terminate":
			_, err = r.Terminate()
			if log.Extras() {
				log.Info("ReaperEvent: Terminating ", r.ReapableDescriptionTiny())
			}
		default:
			log.Error(fmt.Sprintf("Invalid %s Mode %s", e.Config.Name, e.Config.Mode))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// newBatchReapableEvent is a method of EventReporter
func (e *ReaperEvent) newBatchReapableEvent(rs []Reapable, tags []string) error {
	for _, r := range rs {
		err := e.newReapableEvent(r, tags)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetConfig is a method of EventReporter
func (e *ReaperEvent) GetConfig() EventReporterConfig {
	return *e.Config.EventReporterConfig
}

// newCountStatistic is a method of EventReporter
func (e *ReaperEvent) newCountStatistic(string, []string) error {
	return nil
}

// newStatistic is a method of EventReporter
func (e *ReaperEvent) newStatistic(string, float64, []string) error {
	return nil
}

// newEvent is a method of EventReporter
func (e *ReaperEvent) newEvent(string, string, map[string]string, []string) error {
	return nil
}
