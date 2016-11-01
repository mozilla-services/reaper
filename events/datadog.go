package events

import "github.com/PagerDuty/godspeed"

// DatadogConfig is the configuration for a Datadog
type DatadogConfig struct {
	*EventReporterConfig
}

// Datadog is a foundation for DatadogEvents and DatadogStatistics
// uses godspeed, requires dd-agent running
type Datadog struct {
	Config    *DatadogConfig
	_godspeed *godspeed.Godspeed
}

// NewDatadog returns a new instance of Datadog
func NewDatadog(c *DatadogConfig) *Datadog {
	c.Name = "Datadog"
	return &Datadog{Config: c}
}

// setDryRun is a method of EventReporter
// setDryRun sets a Datadog's DryRun value
func (e *Datadog) setDryRun(b bool) {
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

func (e *Datadog) godspeed() (*godspeed.Godspeed, error) {
	if e._godspeed == nil {
		gs, err := godspeed.NewDefault()
		if err != nil {
			return nil, err
		}
		e._godspeed = gs
	}
	return e._godspeed, nil
}
