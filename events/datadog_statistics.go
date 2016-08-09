package events

import log "github.com/mozilla-services/reaper/reaperlog"

// DatadogStatistics implements EventReporter encapsulates Datadog, sends statistics to Datadog
// uses godspeed, requires dd-agent running
type DatadogStatistics struct {
	Datadog
}

// NewDatadogStatistics returns a new instance of Datadog
func NewDatadogStatistics(c *DatadogConfig) *DatadogStatistics {
	c.Name = "DatadogStatistics"
	return &DatadogStatistics{Datadog{Config: c}}
}

// NewStatistic is a method of EventReporter
// NewStatistic reports a gauge to Datadog
func (e *DatadogStatistics) NewStatistic(name string, value float64, tags []string) error {
	if e.Config.DryRun {
		if log.Extras() {
			log.Info("DryRun: Not reporting %s", name)
		}
		return nil
	}

	log.Info("Reporting statistic %s: %d, tags: %v", name, value, tags)

	g, err := e.godspeed()
	if err != nil {
		return err
	}
	err = g.Gauge(name, value, tags)
	return err
}

// NewCountStatistic is a method of EventReporter
// NewCountStatistic reports an Incr to Datadog
func (e *DatadogStatistics) NewCountStatistic(name string, tags []string) error {
	if e.Config.DryRun {
		if log.Extras() {
			log.Info("DryRun: Not reporting %s", name)
		}
		return nil
	}

	log.Info("Reporting count statistic %s", name)

	g, err := e.godspeed()
	if err != nil {
		return err
	}
	err = g.Incr(name, tags)
	return err
}

// NewReapableEvent is a method of EventReporter
func (e *DatadogStatistics) NewReapableEvent(r Reapable, tags []string) error {
	return nil
}

// NewBatchReapableEvent is a method of EventReporter
func (e *DatadogStatistics) NewBatchReapableEvent(rs []Reapable, tags []string) error {
	return nil
}
