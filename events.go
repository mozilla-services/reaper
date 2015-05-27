package reaper

import (
	"fmt"

	"github.com/PagerDuty/godspeed"
	. "github.com/tj/go-debug"
)

var debugEvents = Debug("reaper:events")

type EventReporter interface {
	NewEvent(title string, text string, fields map[string]string, tags []string)
	NewStatistic(name string, value float64, tags []string)
}

type DataDog struct {
}

// TODO: make this async?
// TODO: not recreate godspeed
func (d DataDog) NewEvent(title string, text string, fields map[string]string, tags []string) {
	g, err := godspeed.NewDefault()
	if err != nil {
		debugEvents("Error creating Godspeed, ", err)
	}
	defer g.Conn.Close()
	err = g.Event(title, text, fields, tags)
	if err != nil {
		debugEvents(fmt.Sprintf("reporting Godspeed event %s", title), err)
	}
}

func (d DataDog) NewStatistic(name string, value float64, tags []string) {
	g, err := godspeed.NewDefault()
	if err != nil {
		debugEvents("creating Godspeed, ", err)
	}
	defer g.Conn.Close()
	err = g.Gauge(name, value, tags)
	if err != nil {
		debugEvents(fmt.Sprintf("reporting Godspeed statistic %s", name), err)
	}
}
