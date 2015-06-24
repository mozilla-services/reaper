package events

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/PagerDuty/godspeed"

	log "github.com/milescrabill/reaper/reaperlog"
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

func (d *DataDog) SetNotificationExtras(b bool) {
	d.Config.Extras = b
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
		if d.Config.Extras {
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
		if d.Config.Extras {
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
		if d.Config.Extras {
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

// TODO: make this based on size rather than number of events
func (e *DataDog) NewBatchReapableEvent(rs []Reapable) error {
	var triggering []Reapable
	for _, r := range rs {
		if e.Config.ShouldTriggerFor(r) {
			triggering = append(triggering, r)
		}
	}
	if len(triggering) == 0 {
		return nil
	}
	log.Info("Sending batch DataDog events for %d reapables.", len(triggering))
	// j keeps track of which reapable
	j := 0
	for j < len(triggering) {
		buffer := *bytes.NewBuffer(nil)
		// i keeps track of how many reapables
		// have been written to a buffer
		for j < len(triggering) {
			buffer.ReadFrom(triggering[j].ReapableEventTextShort())
			buffer.WriteString("\n")

			// when we've written 3 reapables
			// move on to the next buffer
			if (j%2 == 0 && j != 0) || j == len(triggering)-1 {
				// send events in this buffer
				err := e.NewEvent("Reapable resources discovered", buffer.String(), nil, nil)
				if err != nil {
					return err
				}
				j++
				break
			}
			j++
		}
	}

	return nil
}
