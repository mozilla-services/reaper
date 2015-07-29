package events

import (
	"bytes"
	"fmt"
	"net/mail"

	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

type NotificationsConfig struct {
	state.StatesConfig
}

type Reapable interface {
	reapable.Reapable
	ReapableEventText() *bytes.Buffer
	ReapableEventTextShort() *bytes.Buffer
	ReapableEventEmail() (mail.Address, string, *bytes.Buffer, error)
	ReapableEventEmailShort() (mail.Address, *bytes.Buffer, error)
}

type EventReporterConfig struct {
	Enabled bool
	DryRun  bool

	Name string

	// should be []state.StateEnum...
	Triggers []string
}

func (e *EventReporterConfig) ParseTriggers() (triggers []state.StateEnum) {
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

func (e *EventReporterConfig) ShouldTriggerFor(r Reapable) bool {
	triggering := false
	// if the reapable's state is set to trigger this EventReporter
	for _, trigger := range e.ParseTriggers() {
		// if the reapable's state should trigger this event and the state was just updated
		if trigger == r.ReaperState().State && r.ReaperState().Updated {
			triggering = true
		}
	}

	if e.DryRun {
		if log.Extras() {
			log.Notice("DryRun: Not triggering %s for %s", e.Name, r.ReapableDescriptionTiny())
		}
		return false
	}
	return triggering
}

type EventReporter interface {
	NewEvent(title string, text string, fields map[string]string, tags []string) error
	NewStatistic(name string, value float64, tags []string) error
	NewCountStatistic(name string, tags []string) error
	NewReapableEvent(r Reapable, tags []string) error
	NewBatchReapableEvent(rs []Reapable, tags []string) error
	SetDryRun(b bool)
	Cleanup() error
}

// implements EventReporter but does nothing
type NoEventReporter struct {
	EventReporterConfig
}

// TODO: this is sorta redundant with triggers, won't ever activate
// not that it ever did...
func NewNoEventReporter() *NoEventReporter {
	return &NoEventReporter{EventReporterConfig{Name: "NoEventReporter"}}
}

func (*NoEventReporter) NewEvent(title string, text string, fields map[string]string, tags []string) error {
	return nil
}
func (*NoEventReporter) NewStatistic(name string, value float64, tags []string) error {
	return nil
}
func (*NoEventReporter) NewCountStatistic(name string, tags []string) error {
	return nil
}

func (n *NoEventReporter) NewReapableEvent(r Reapable, tags []string) error {
	return nil
}

func (n *NoEventReporter) NewBatchReapableEvent(rs []Reapable, tags []string) error {
	return nil
}

func (*NoEventReporter) SetDryRun(b bool) {}

func (*NoEventReporter) Cleanup() error { return nil }

// TODO: this is sorta redundant with triggers, won't ever activate
type ErrorEventReporter struct {
	EventReporterConfig
}

func NewErrorEventReporter() *ErrorEventReporter {
	return &ErrorEventReporter{EventReporterConfig{Name: "ErrorEventReporter"}}
}

func (*ErrorEventReporter) NewEvent(title string, text string, fields map[string]string, tags []string) error {
	return fmt.Errorf("ErrorEventReporter")
}

func (*ErrorEventReporter) NewStatistic(name string, value float64, tags []string) error {
	return fmt.Errorf("ErrorEventReporter")
}

func (*ErrorEventReporter) NewCountStatistic(name string, tags []string) error {
	return fmt.Errorf("ErrorEventReporter")
}

func (*ErrorEventReporter) Cleanup() error {
	return fmt.Errorf("ErrorEventReporter")
}

func (e *ErrorEventReporter) NewReapableEvent(r Reapable, tags []string) error {
	return fmt.Errorf("ErrorEventReporter")
}

func (e *ErrorEventReporter) NewBatchReapableEvent(rs []Reapable, tags []string) error {
	return fmt.Errorf("ErrorEventReporter")
}

func (*ErrorEventReporter) SetDryRun(b bool) {}
