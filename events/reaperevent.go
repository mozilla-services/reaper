package events

import (
	"time"

	"github.com/mostlygeek/reaper/state"
)

type ReaperEventConfig struct {
	Enabled bool
	DryRun  bool
}

type ReaperEvent struct {
	Config *ReaperEventConfig
}

func NewReaperEvent(c *ReaperEventConfig) *ReaperEvent {
	return &ReaperEvent{c}
}

func (e *ReaperEvent) NewEvent(title string, text string, fields map[string]string, tags []string) error {
	return nil
}
func (e *ReaperEvent) NewStatistic(name string, value float64, tags []string) error {
	return nil
}
func (e *ReaperEvent) NewCountStatistic(name string, tags []string) error {
	return nil
}
func (e *ReaperEvent) NewReapableEvent(r Reapable) error {
	// this only gets called if ReaperEvent is added, so we check
	// for dryrun, that we have passed NOTIFY2, and that current time is
	// later than the Until time
	if !e.Config.DryRun && e.Config.Enabled &&
		time.Now().After(r.ReaperState().Until) &&
		r.ReaperState().State == state.STATE_REAPABLE {
		_, err := r.Stop()
		if err != nil {
			return err
		}
	}
	return nil
}
