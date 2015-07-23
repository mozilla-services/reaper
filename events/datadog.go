package events

import (
	"bytes"
	"fmt"
	"strconv"
	"sync"

	"github.com/PagerDuty/godspeed"

	log "github.com/mozilla-services/reaper/reaperlog"
)

// DataDogConfig is the configuration for a DataDog
type DataDogConfig struct {
	EventReporterConfig
	Host string
	Port string
}

// DataDog implements EventReporter, sends events and statistics to DataDog
// uses godspeed, requires dd-agent running
type DataDog struct {
	Config    *DataDogConfig
	_godspeed *godspeed.Godspeed
	sync.Once
}

func NewDataDog(c *DataDogConfig) *DataDog {
	c.Name = "DataDog"
	return &DataDog{Config: c}
}

func (d *DataDog) SetDryRun(b bool) {
	d.Config.DryRun = b
}

func (e *DataDog) Cleanup() error {
	g, err := e.godspeed()
	if err != nil {
		return err
	}
	err = g.Conn.Close()
	return err
}

func (d *DataDog) Do() {
	var g *godspeed.Godspeed
	var err error
	// if config options not set, use defaults
	if d.Config.Host == "" || d.Config.Port == "" {
		g, err = godspeed.NewDefault()
	} else {
		port, err := strconv.Atoi(d.Config.Port)
		if err != nil {
			log.Error(err.Error())
		}
		g, err = godspeed.New(d.Config.Host, port, false)
	}
	if err != nil {
		log.Error(err.Error())
	}
	d._godspeed = g
}

func (d *DataDog) godspeed() (*godspeed.Godspeed, error) {
	if d._godspeed == nil {
		d.Do()
	}
	return d._godspeed, nil
}

// NewEvent reports an event to DataDog
func (d *DataDog) NewEvent(title string, text string, fields map[string]string, tags []string) error {
	if d.Config.DryRun {
		if log.Extras() {
			log.Notice("DryRun: Not reporting %s", title)
		}
		return nil
	}

	g, err := d.godspeed()
	if err != nil {
		return err
	}
	err = g.Event(title, text, fields, tags)
	if err != nil {
		return err
	}
	return nil
}

// NewStatistic reports a gauge to DataDog
func (d *DataDog) NewStatistic(name string, value float64, tags []string) error {
	if d.Config.DryRun {
		if log.Extras() {
			log.Notice("DryRun: Not reporting %s", name)
		}
		return nil
	}

	g, err := d.godspeed()
	if err != nil {
		return err
	}
	err = g.Gauge(name, value, tags)
	return err
}

// NewCountStatistic reports an Incr to DataDog
func (d *DataDog) NewCountStatistic(name string, tags []string) error {
	if d.Config.DryRun {
		if log.Extras() {
			log.Notice("DryRun: Not reporting %s", name)
		}
		return nil
	}

	g, err := d.godspeed()
	if err != nil {
		return err
	}
	err = g.Incr(name, tags)
	return err
}

// NewReapableEvent is shorthand for a NewEvent about a reapable resource
func (d *DataDog) NewReapableEvent(r Reapable, tags []string) error {
	if d.Config.ShouldTriggerFor(r) {
		err := d.NewEvent("Reapable resource discovered", string(r.ReapableEventText().Bytes()), nil, append(tags, "id:%s", r.ReapableDescriptionTiny()))
		if err != nil {
			return fmt.Errorf("Error reporting Reapable event for %s", r.ReapableDescriptionTiny())
		}
	}
	return nil
}

func (e *DataDog) NewBatchReapableEvent(rs []Reapable, tags []string) error {
	var triggering []Reapable
	for _, r := range rs {
		if e.Config.ShouldTriggerFor(r) {
			triggering = append(triggering, r)
		}
	}
	// no events triggering
	if len(triggering) == 0 {
		return nil
	}

	log.Info("Sending batch DataDog events for %d reapables.", len(triggering))
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
