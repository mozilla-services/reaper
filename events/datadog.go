package events

import (
	"bytes"
	"fmt"
	"strconv"
	"sync"

	"github.com/PagerDuty/godspeed"

	log "github.com/mozilla-services/reaper/reaperlog"
)

// DatadogConfig is the configuration for a Datadog
type DatadogConfig struct {
	eventReporterConfig
	Host string
	Port string
}

// Datadog implements EventReporter, sends events and statistics to Datadog
// uses godspeed, requires dd-agent running
type Datadog struct {
	Config    *DatadogConfig
	_godspeed *godspeed.Godspeed
	sync.Once
}

// NewDatadog returns a new instance of Datadog
func NewDatadog(c *DatadogConfig) *Datadog {
	c.Name = "Datadog"
	return &Datadog{Config: c}
}

// SetDryRun is a method of EventReporter
// SetDryRun sets a Datadog's DryRun value
func (e *Datadog) SetDryRun(b bool) {
	e.Config.DryRun = b
}

// Cleanup is a method of EventReporter
// Cleanup performs any actions necessary to clean up after a Datadog
func (e *Datadog) Cleanup() error {
	g, err := e.godspeed()
	if err != nil {
		return err
	}
	err = g.Conn.Close()
	return err
}

// Do is a method of sync.Once
func (e *Datadog) getGodspeed() {
	var gs *godspeed.Godspeed
	var err error
	// if config options not set, use defaults
	if e.Config.Host == "" || e.Config.Port == "" {
		gs, err = godspeed.NewDefault()
	} else {
		port, err := strconv.Atoi(e.Config.Port)
		if err != nil {
			log.Error(err.Error())
		}
		gs, err = godspeed.New(e.Config.Host, port, false)
	}
	if err != nil {
		log.Error(err.Error())
	}
	e._godspeed = gs
}

func (e *Datadog) godspeed() (*godspeed.Godspeed, error) {
	if e._godspeed == nil {
		e.Do(e.getGodspeed)
	}
	return e._godspeed, nil
}

// NewEvent is a method of EventReporter
// NewEvent reports an event to Datadog
func (e *Datadog) NewEvent(title string, text string, fields map[string]string, tags []string) error {
	if e.Config.DryRun {
		if log.Extras() {
			log.Notice("DryRun: Not reporting %s", title)
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

// NewStatistic is a method of EventReporter
// NewStatistic reports a gauge to Datadog
func (e *Datadog) NewStatistic(name string, value float64, tags []string) error {
	if e.Config.DryRun {
		if log.Extras() {
			log.Notice("DryRun: Not reporting %s", name)
		}
		return nil
	}

	g, err := e.godspeed()
	if err != nil {
		return err
	}
	err = g.Gauge(name, value, tags)
	return err
}

// NewCountStatistic is a method of EventReporter
// NewCountStatistic reports an Incr to Datadog
func (e *Datadog) NewCountStatistic(name string, tags []string) error {
	if e.Config.DryRun {
		if log.Extras() {
			log.Notice("DryRun: Not reporting %s", name)
		}
		return nil
	}

	g, err := e.godspeed()
	if err != nil {
		return err
	}
	err = g.Incr(name, tags)
	return err
}

// NewReapableEvent is a method of EventReporter
// NewReapableEvent is shorthand for a NewEvent about a reapable resource
func (e *Datadog) NewReapableEvent(r Reapable, tags []string) error {
	if e.Config.shouldTriggerFor(r) {
		err := e.NewEvent("Reapable resource discovered", string(r.ReapableEventText().Bytes()), nil, append(tags, "id:%s", r.ReapableDescriptionTiny()))
		if err != nil {
			return fmt.Errorf("Error reporting Reapable event for %s", r.ReapableDescriptionTiny())
		}
	}
	return nil
}

// NewBatchReapableEvent is a method of EventReporter
func (e *Datadog) NewBatchReapableEvent(rs []Reapable, tags []string) error {
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
			text := triggering[j].ReapableEventTextShort()
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
