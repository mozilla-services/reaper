package events

import (
	"bytes"
	"fmt"
	"strconv"

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
}

func NewDataDog(c *DataDogConfig) *DataDog {
	c.Name = "DataDog"
	return &DataDog{Config: c}
}

func (d *DataDog) SetDryRun(b bool) {
	d.Config.DryRun = b
}

// TODO: make this async?
// TODO: don't recreate godspeed
func (d *DataDog) godspeed() (*godspeed.Godspeed, error) {
	if d._godspeed == nil {
		var g *godspeed.Godspeed
		var err error
		// if config options not set, use defaults
		if d.Config.Host == "" || d.Config.Port == "" {
			g, err = godspeed.NewDefault()
		} else {
			port, err := strconv.Atoi(d.Config.Port)
			if err != nil {
				return nil, err
			}
			g, err = godspeed.New(d.Config.Host, port, false)
		}
		if err != nil {
			return nil, err
		}
		d._godspeed = g
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
	// TODO: fix?
	// defer g.Conn.Close()
	err = g.Event(title, text, fields, tags)
	if err != nil {
		return fmt.Errorf("Error reporting Godspeed event %s: %s", title, err)
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
	// TODO: fix?
	// defer g.Conn.Close()
	err = g.Gauge(name, value, tags)
	if err != nil {
		return fmt.Errorf("Error reporting Godspeed statistic %s: %s", name, err)
	}

	return nil
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
	// TODO: fix?
	// defer g.Conn.Close()
	err = g.Incr(name, tags)
	if err != nil {
		return fmt.Errorf("Error reporting Godspeed count statistic %s: %s", name, err)
	}
	return nil
}

// NewReapableEvent is shorthand for a NewEvent about a reapable resource
func (d *DataDog) NewReapableEvent(r Reapable) error {
	if d.Config.ShouldTriggerFor(r) {
		err := d.NewEvent("Reapable resource discovered", string(r.ReapableEventText().Bytes()), nil, []string{fmt.Sprintf("id:%s", r.ReapableDescriptionTiny())})
		if err != nil {
			return fmt.Errorf("Error reporting Reapable event for %s", r.ReapableDescriptionTiny())
		}
	}
	return nil
}

func (e *DataDog) NewBatchReapableEvent(rs []Reapable) error {
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
		var written int64 = 0
		buffer := *bytes.NewBuffer(nil)
		for moveOn := false; j < len(triggering) && !moveOn; {
			// if there is room to write another reapable
			size := int64(triggering[j].ReapableEventTextShort().Len())
			log.Info("Written: %d, Size: %d", written, size)
			if size+written < 4000 {
				// write it + a newline
				n, err := buffer.ReadFrom(triggering[j].ReapableEventTextShort())
				_, err = buffer.WriteString("\n")
				if err != nil {
					return err
				}
				written += n
				// increment counter of written reapables
				j++
			} else {
				// no room for another reapable
				// send events in this buffer
				err := e.NewEvent("Reapable resources discovered", buffer.String(), nil, nil)
				if err != nil {
					return err
				}
				moveOn = true
			}
		}
	}

	return nil
}
