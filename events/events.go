package events

import (
	"bytes"
	"net/mail"

	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

// NotificationsConfig wraps state.StatesConfig
type NotificationsConfig struct {
	state.StatesConfig
}

// Reapable expands upon the reapable.Reapable interface
type Reapable interface {
	reapable.Reapable
	ReapableEventText() (*bytes.Buffer, error)
	ReapableEventTextShort() (*bytes.Buffer, error)
	ReapableEventEmail() (mail.Address, string, *bytes.Buffer, error)
	ReapableEventEmailShort() (mail.Address, *bytes.Buffer, error)
}

// EventReporterConfig has configuration variables for EventReporters
type EventReporterConfig struct {
	Enabled bool
	DryRun  bool

	Name string

	// string representations of states from state.StateEnum
	Triggers []string
}

func (e *EventReporterConfig) parseTriggers() (triggers []state.StateEnum) {
	for _, t := range e.Triggers {
		switch t {
		case "first":
			triggers = append(triggers, state.FirstState)
		case "second":
			triggers = append(triggers, state.SecondState)
		case "third":
			triggers = append(triggers, state.ThirdState)
		case "final":
			triggers = append(triggers, state.FinalState)
		case "ignore":
			triggers = append(triggers, state.IgnoreState)
		default:
			log.Warning("%s is not an available EventReporter trigger", t)
		}
	}
	return
}

func (e *EventReporterConfig) shouldTriggerFor(r Reapable) bool {
	triggering := false
	// if the reapable's state is set to trigger this EventReporter
	for _, trigger := range e.parseTriggers() {
		// if the reapable's state should trigger this event and the state was just updated
		if trigger == r.ReaperState().State && r.ReaperState().Updated {
			triggering = true
		}
	}

	if e.DryRun {
		if log.Extras() {
			log.Info("DryRun: Not triggering %s for %s", e.Name, r.ReapableDescriptionTiny())
		}
		return false
	}
	return triggering
}

// Cleaner needs to be cleaned up
type Cleaner interface {
	Cleanup() error
}

// EventReporter contains different event and statistics reporting
// embeds ReapableEventReporter
type EventReporter interface {
	NewEvent(title string, text string, fields map[string]string, tags []string) error
	NewStatistic(name string, value float64, tags []string) error
	NewCountStatistic(name string, tags []string) error
	NewReapableEvent(r Reapable, tags []string) error
	NewBatchReapableEvent(rs []Reapable, tags []string) error
	SetDryRun(b bool)
	GetConfig() EventReporterConfig
}
