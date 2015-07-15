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
	Config *DataDogConfig
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

// TODO: make this use sync.Once?
func (d *DataDog) godspeed() (*godspeed.AsyncGodspeed, error) {
	var g *godspeed.AsyncGodspeed
	var err error
	// if config options not set, use defaults
	if d.Config.Host == "" || d.Config.Port == "" {
		g, err = godspeed.NewDefaultAsync()
	} else {
		port, err := strconv.Atoi(d.Config.Port)
		if err != nil {
			return nil, err
		}
		g, err = godspeed.NewAsync(d.Config.Host, port, false)
	}
	if err != nil {
		return nil, err
	}
	return g, nil
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
	defer g.Godspeed.Conn.Close()
	g.W.Add(1)
	go g.Event(title, text, fields, tags, g.W)
	g.W.Wait()
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
	defer g.Godspeed.Conn.Close()
	g.W.Add(1)
	go g.Gauge(name, value, tags, g.W)
	g.W.Wait()
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
	defer g.Godspeed.Conn.Close()
	g.W.Add(1)
	go g.Incr(name, tags, g.W)
	g.W.Wait()
	return nil
}

// NewReapableEvent is shorthand for a NewEvent about a reapable resource
func (d *DataDog) NewReapableEvent(r Reapable) error {
	if d.Config.ShouldTriggerFor(r) {
		d.NewEvent("Reapable resource discovered", string(r.ReapableEventText().Bytes()), nil, []string{fmt.Sprintf("id:%s", r.ReapableDescriptionTiny())})
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
		var written int64
		buffer := *bytes.NewBuffer(nil)
		for moveOn := false; j < len(triggering) && !moveOn; {
			size := int64(triggering[j].ReapableEventTextShort().Len())

			// if there is room
			if size+written < 4500 {
				// write it + a newline
				n, err := buffer.ReadFrom(triggering[j].ReapableEventTextShort())
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
		err := e.NewEvent("Reapable resources discovered", buffer.String(), nil, nil)
		if err != nil {
			return err
		}
	}

	return nil
}
