package events

import (
	"time"

	"github.com/mostlygeek/reaper/state"
)

type ReaperEventConfig struct {
	Enabled bool
	DryRun  bool
	Extras  bool
}

type ReaperEvent struct {
	Config *ReaperEventConfig
}

func (e *ReaperEvent) SetDryRun(b bool) {
	e.Config.DryRun = b
}

func (e *ReaperEvent) SetNotificationExtras(b bool) {
	e.Config.Extras = b
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
	if e.Config.DryRun && e.Config.Extras {
		log.Notice("DryRun: Not mailing about %s", r.ReapableDescription())
		return nil
	}

	// this only gets called if ReaperEvent is added, so we check
	// for dryrun, that the reapable is in STATE_REAPABLE,
	// and that current time is later than its Until time
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
