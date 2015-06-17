package events

import (
	"bytes"
	"fmt"
	"net/mail"

	"github.com/milescrabill/reaper/reapable"
	log "github.com/milescrabill/reaper/reaperlog"
	"github.com/milescrabill/reaper/state"
)

type NotificationsConfig struct {
	state.StatesConfig
	Extras bool
}

type Reapable interface {
	reapable.Reapable
	ReapableEventText() *bytes.Buffer
	ReapableEventTextShort() *bytes.Buffer
	//ReapableEventHTML() *bytes.Buffer
	ReapableEventEmail() (mail.Address, string, string, error)
}

type EventReporterConfig struct {
	Enabled bool
	DryRun  bool
	Extras  bool

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
		if trigger == r.ReaperState().State {
			triggering = true
		}
	}

	if e.DryRun {
		if e.Extras {
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
	NewReapableEvent(r Reapable) error
	NewBatchReapableEvent(rs []Reapable) error
	SetDryRun(b bool)
	SetNotificationExtras(b bool)
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

func (n *NoEventReporter) NewEvent(title string, text string, fields map[string]string, tags []string) error {
	return nil
}
func (n *NoEventReporter) NewStatistic(name string, value float64, tags []string) error {
	return nil
}
func (n *NoEventReporter) NewCountStatistic(name string, tags []string) error {
	return nil
}

func (n *NoEventReporter) NewReapableEvent(r Reapable) error {
	return nil
}

func (n *NoEventReporter) SetDryRun(b bool)             {}
func (n *NoEventReporter) SetNotificationExtras(b bool) {}

// TODO: this is sorta redundant with triggers, won't ever activate
type ErrorEventReporter struct {
	EventReporterConfig
}

func NewErrorEventReporter() *ErrorEventReporter {
	return &ErrorEventReporter{EventReporterConfig{Name: "ErrorEventReporter"}}
}

func (e *ErrorEventReporter) NewEvent(title string, text string, fields map[string]string, tags []string) error {
	return fmt.Errorf("ErrorEventReporter")
}

func (e *ErrorEventReporter) NewStatistic(name string, value float64, tags []string) error {
	return fmt.Errorf("ErrorEventReporter")
}

func (e *ErrorEventReporter) NewCountStatistic(name string, tags []string) error {
	return fmt.Errorf("ErrorEventReporter")
}

func (e *ErrorEventReporter) NewReapableEvent(r Reapable) error {
	return fmt.Errorf("ErrorEventReporter")
}

func (e *ErrorEventReporter) SetDryRun(b bool)             {}
func (e *ErrorEventReporter) SetNotificationExtras(b bool) {}
