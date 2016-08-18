package events

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	log "github.com/mozilla-services/reaper/reaperlog"
)

// DatadogEvents implements EventReporter encapsulates Datadog, sends events to Datadog
// uses godspeed, requires dd-agent running
type DatadogEvents struct {
	Datadog
}

// NewDatadogEvents returns a new instance of DatadogEvents
func NewDatadogEvents(c *DatadogConfig) *DatadogEvents {
	c.Name = "DatadogEvents"
	return &DatadogEvents{Datadog{Config: c}}
}

// newEvent is a method of EventReporter
// newEvent reports an event to Datadog
func (e *DatadogEvents) newEvent(title string, text string, fields map[string]string, tags []string) error {
	if e.Config.DryRun {
		if log.Extras() {
			log.Info("DryRun: Not reporting %s", title)
		}
		return nil
	}

	g, err := e.godspeed()
	if err != nil {
		return err
	}
	err = g.Event(title, text, fields, tags)
	if err != nil {
		return err
	}
	return nil
}

// newReapableEvent is a method of EventReporter
// newReapableEvent is shorthand for a newEvent about a reapable resource
func (e *DatadogEvents) newReapableEvent(r Reapable, tags []string) error {
	if e.Config.shouldTriggerFor(r) {
		text, err := r.ReapableEventText()
		if err != nil {
			return err
		}
		err = e.newEvent("Reapable resource discovered", text.String(), nil, tags)
		if err != nil {
			return fmt.Errorf("Error reporting Reapable event for %s: %s", r.ReapableDescriptionTiny(), err.Error())
		}
	}
	return nil
}

// newBatchReapableEvent is a method of EventReporter
func (e *DatadogEvents) newBatchReapableEvent(rs []Reapable, tags []string) error {
	errorStrings := []string{}
	buffer := new(bytes.Buffer)
	for _, r := range rs {
		if !e.Config.shouldTriggerFor(r) {
			continue
		}
		text, err := r.ReapableEventTextShort()
		if err != nil {
			errorStrings = append(errorStrings, fmt.Sprintf("ReapableEventText: %v", err))
			continue
		}
		if text.Len() > 4500 {
			text.Truncate(4499)
		}
		if text.Len()+buffer.Len() > 4500 {
			// send events in this buffer
			err := e.newEvent("Reapable resources discovered", buffer.String(), nil, tags)
			if err != nil {
				errorStrings = append(errorStrings, fmt.Sprintf("newEvent: %v", err))
			}
			buffer.Reset()
		}
		buffer.ReadFrom(text)
		buffer.WriteByte('\n')
	}

	// Flush remaining buffer
	if buffer.Len() > 0 {
		// send events in this buffer
		err := e.newEvent("Reapable resources discovered", buffer.String(), nil, tags)
		if err != nil {
			errorStrings = append(errorStrings, fmt.Sprintf("newEvent: %v", err))
		}
	}
	if len(errorStrings) > 0 {
		return errors.New(strings.Join(errorStrings, "\n"))
	}
	return nil
}

// newCountStatistic is a method of EventReporter
func (e *DatadogEvents) newCountStatistic(string, []string) error {
	return nil
}

// newStatistic is a method of EventReporter
func (e *DatadogEvents) newStatistic(string, float64, []string) error {
	return nil
}

// GetConfig is a method of EventReporter
func (e *DatadogEvents) GetConfig() EventReporterConfig {
	return *e.Config.EventReporterConfig
}
