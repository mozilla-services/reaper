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

// SetDryRun is a method of EventReporter
func (e *ReaperEvent) SetDryRun(b bool) {
	e.Config.DryRun = b
}

// NewReaperEvent returns a new instance of ReaperEvent
func NewReaperEvent(c *ReaperEventConfig) *ReaperEvent {
	c.Name = "ReaperEvent"
	return &ReaperEvent{c}
}

// NewReapableEvent is a method of EventReporter
func (e *ReaperEvent) NewReapableEvent(r Reapable, tags []string) error {
	if e.Config.shouldTriggerFor(r) {
		if log.Extras() {
			log.Error("Triggering ReaperEvent for ", r.ReaperState().String())
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

// NewBatchReapableEvent is a method of EventReporter
func (e *ReaperEvent) NewBatchReapableEvent(rs []Reapable, tags []string) error {
	for _, r := range rs {
		err := e.NewReapableEvent(r, tags)
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

// NewCountStatistic is a method of EventReporter
func (e *ReaperEvent) NewCountStatistic(string, []string) error {
	return nil
}

// NewStatistic is a method of EventReporter
func (e *ReaperEvent) NewStatistic(string, float64, []string) error {
	return nil
}

// NewEvent is a method of EventReporter
func (e *ReaperEvent) NewEvent(string, string, map[string]string, []string) error {
	return nil
}
