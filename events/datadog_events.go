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

// NewEvent is a method of EventReporter
// NewEvent reports an event to Datadog
func (e *DatadogEvents) NewEvent(title string, text string, fields map[string]string, tags []string) error {
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

// NewReapableEvent is a method of EventReporter
// NewReapableEvent is shorthand for a NewEvent about a reapable resource
func (e *DatadogEvents) NewReapableEvent(r Reapable, tags []string) error {
	if e.Config.shouldTriggerFor(r) {
		text, err := r.ReapableEventText()
		if err != nil {
			return err
		}
		err = e.NewEvent("Reapable resource discovered", text.String(), nil, append(tags, "id:%s", r.ReapableDescriptionTiny()))
		if err != nil {
			return fmt.Errorf("Error reporting Reapable event for %s: %s", r.ReapableDescriptionTiny(), err.Error())
		}
	}
	return nil
}

// NewBatchReapableEvent is a method of EventReporter
func (e *DatadogEvents) NewBatchReapableEvent(rs []Reapable, tags []string) error {
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
			err := e.NewEvent("Reapable resources discovered", buffer.String(), nil, tags)
			if err != nil {
				errorStrings = append(errorStrings, fmt.Sprintf("NewEvent: %v", err))
			}
			buffer.Reset()
		}
		buffer.ReadFrom(text)
		buffer.WriteByte('\n')
	}

	// Flush remaining buffer
	if buffer.Len() > 0 {
		// send events in this buffer
		err := e.NewEvent("Reapable resources discovered", buffer.String(), nil, tags)
		if err != nil {
			errorStrings = append(errorStrings, fmt.Sprintf("NewEvent: %v", err))
		}
	}
	if len(errorStrings) > 0 {
		return errors.New(strings.Join(errorStrings, "\n"))
	}
	return nil
}

// NewCountStatistic is a method of EventReporter
func (e *DatadogEvents) NewCountStatistic(string, []string) error {
	return nil
}

// NewStatistic is a method of EventReporter
func (e *DatadogEvents) NewStatistic(string, float64, []string) error {
	return nil
}

// GetConfig is a method of EventReporter
func (e *DatadogEvents) GetConfig() EventReporterConfig {
	return *e.Config.EventReporterConfig
}
