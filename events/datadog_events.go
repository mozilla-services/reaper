package events

import (
	"bytes"
	"fmt"

	log "github.com/mozilla-services/reaper/reaperlog"
)

// DatadogStatistics implements EventReporter encapsulates DatadogEvents, sends events to Datadog
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
	var triggering []Reapable
	for _, r := range rs {
		if e.Config.shouldTriggerFor(r) {
			triggering = append(triggering, r)
		}
	}
	// no events triggering
	if len(triggering) == 0 {
		return nil
	}

	log.Info("Sending batch Datadog events for %d reapables.", len(triggering))
	// this is a bin packing problem
	// we ignore its complexity because we don't care (that much)
	for j := 0; j < len(triggering); {
		var written int64
		buffer := *bytes.NewBuffer(nil)
		for moveOn := false; j < len(triggering) && !moveOn; {
			text, err := triggering[j].ReapableEventTextShort()
			if err != nil {
				return err
			}
			size := int64(text.Len())

			// if there is room
			if size+written < 4500 {
				// write it + a newline
				n, err := buffer.ReadFrom(text)
				// not counting this length, but we have padding
				_, err = buffer.WriteString("\n")
				if err != nil {
					return err
				}
				written += n
				// increment counter of written reapables
				j++
			} else {
				// if we've written enough to the buffer, break the loop
				moveOn = true
			}
		}
		// send events in this buffer
		err := e.NewEvent("Reapable resources discovered", buffer.String(), nil, tags)
		if err != nil {
			return err
		}
	}

	return nil
}
