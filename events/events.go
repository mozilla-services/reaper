package events

import (
	"bytes"
	"errors"
	"net/mail"
	"strings"

	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

var eventReporters *[]EventReporter

func SetEvents(e *[]EventReporter) {
	eventReporters = e
}

func SetDryRun(DryRun bool) {
	// set config values for events
	for _, er := range *eventReporters {
		er.setDryRun(DryRun)
	}
}

func Cleanup() {
	for _, er := range *eventReporters {
		c, ok := er.(Cleaner)
		if ok {
			if err := c.Cleanup(); err != nil {
				log.Error(err.Error())
			}
		}
	}
}

func NewEvent(title string, text string, fields map[string]string, tags []string) error {
	errorStrings := []string{}
	for _, er := range *eventReporters {
		err := er.newEvent(title, text, fields, tags)
		errorStrings = append(errorStrings, err.Error())
	}
	if len(errorStrings) > 0 {
		return errors.New(strings.Join(errorStrings, "\n"))
	}
	return nil
}

func NewStatistic(name string, value float64, tags []string) error {
	errorStrings := []string{}
	for _, er := range *eventReporters {
		err := er.newStatistic(name, value, tags)
		errorStrings = append(errorStrings, err.Error())
	}
	if len(errorStrings) > 0 {
		return errors.New(strings.Join(errorStrings, "\n"))
	}
	return nil
}

func NewCountStatistic(name string, tags []string) error {
	errorStrings := []string{}
	for _, er := range *eventReporters {
		err := er.newCountStatistic(name, tags)
		errorStrings = append(errorStrings, err.Error())
	}
	if len(errorStrings) > 0 {
		return errors.New(strings.Join(errorStrings, "\n"))
	}
	return nil
}

func NewReapableEvent(r Reapable, tags []string) error {
	errorStrings := []string{}
	for _, er := range *eventReporters {
		err := er.newReapableEvent(r, tags)
		errorStrings = append(errorStrings, err.Error())
	}
	if len(errorStrings) > 0 {
		return errors.New(strings.Join(errorStrings, "\n"))
	}
	return nil
}

func NewBatchReapableEvent(rs []Reapable, tags []string) error {
	errorStrings := []string{}
	for _, er := range *eventReporters {
		err := er.newBatchReapableEvent(rs, tags)
		errorStrings = append(errorStrings, err.Error())
	}
	if len(errorStrings) > 0 {
		return errors.New(strings.Join(errorStrings, "\n"))
	}
	return nil
}

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
// embeds EventReporter
type EventReporter interface {
	newEvent(title string, text string, fields map[string]string, tags []string) error
	newStatistic(name string, value float64, tags []string) error
	newCountStatistic(name string, tags []string) error
	newReapableEvent(r Reapable, tags []string) error
	newBatchReapableEvent(rs []Reapable, tags []string) error
	setDryRun(b bool)
	GetConfig() EventReporterConfig
}
