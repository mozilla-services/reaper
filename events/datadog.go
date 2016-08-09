package events

import (
	"strconv"
	"sync"

	"github.com/PagerDuty/godspeed"

	log "github.com/mozilla-services/reaper/reaperlog"
)

// DatadogConfig is the configuration for a Datadog
type DatadogConfig struct {
	*EventReporterConfig
	Host string
	Port string
}

// Datadog is a foundation for DatadogEvents and DatadogStatistics
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
		// Do is a method of sync.Once
		// this will only get called once
		e.Do(e.getGodspeed)
	}
	return e._godspeed, nil
}
