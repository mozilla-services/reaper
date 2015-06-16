package events

import (
	"bytes"
	"fmt"
	"net/mail"
	"time"

	"github.com/milescrabill/reaper/reapable"
	"github.com/milescrabill/reaper/state"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) (err error) {
	d.Duration, err = time.ParseDuration(string(text))
	return
}

type NotificationsConfig struct {
	Extras             bool
	Interval           Duration // like cron, how often to check instances for reaping
	FirstNotification  Duration // how long after start to first notification
	SecondNotification Duration // how long after notify1 to second notification
	Terminate          Duration // how long after notify2 to terminate
}

type Reapable interface {
	reapable.Reapable
	ReapableEventText() *bytes.Buffer
	//ReapableEventHTML() *bytes.Buffer
	ReapableEventEmail() (mail.Address, string, string, error)
}

type EventReporterConfig struct {
	Enabled bool
	DryRun  bool
	Extras  bool

	// should be []state.StateEnum...
	Triggers []string
}

func (e *EventReporterConfig) ParseTriggers() (triggers []state.StateEnum) {
	for _, t := range e.Triggers {
		switch t {
		case "start":
			triggers = append(triggers, state.STATE_START)
		case "notify1":
			triggers = append(triggers, state.STATE_NOTIFY1)
		case "notify2":
			triggers = append(triggers, state.STATE_NOTIFY2)
		case "reapable":
			triggers = append(triggers, state.STATE_REAPABLE)
		case "whitelist":
			triggers = append(triggers, state.STATE_WHITELIST)
		}
	}
	return
}

func (e *EventReporterConfig) Triggering(r Reapable) bool {
	// if the reapable's state is set to trigger this EventReporter
	triggering := false
	for trigger := range e.Triggers {
		if r.ReaperState().State == state.StateEnum(trigger) {
			triggering = true
		}
	}
	return triggering
}

type EventReporter interface {
	NewEvent(title string, text string, fields map[string]string, tags []string) error
	NewStatistic(name string, value float64, tags []string) error
	NewCountStatistic(name string, tags []string) error
	NewReapableEvent(r Reapable) error
	SetDryRun(b bool)
	SetNotificationExtras(b bool)
}

// implements EventReporter but does nothing
type NoEventReporter struct{}

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

type ErrorEventReporter struct{}

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
