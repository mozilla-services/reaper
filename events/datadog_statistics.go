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

// newStatistic is a method of EventReporter
// newStatistic reports a gauge to Datadog
func (e *DatadogStatistics) newStatistic(name string, value float64, tags []string) error {
	if e.Config.DryRun {
		if log.Extras() {
			log.Info("DryRun: Not reporting %s", name)
		}
		return nil
	}
	if log.Extras() {
		log.Info("DatadogStatistics: reporting %s: %f, tags: %v", name, value, tags)
	}
	g, err := e.godspeed()
	if err != nil {
		return err
	}
	err = g.Gauge(name, value, tags)
	return err
}

// newCountStatistic is a method of EventReporter
// newCountStatistic reports an Incr to Datadog
func (e *DatadogStatistics) newCountStatistic(name string, tags []string) error {
	if e.Config.DryRun {
		if log.Extras() {
			log.Info("DryRun: Not reporting %s", name)
		}
		return nil
	}

	if log.Extras() {
		log.Info("DatadogStatistics: reporting %s, tags: %v", name, tags)
	}

	g, err := e.godspeed()
	if err != nil {
		return err
	}
	err = g.Incr(name, tags)
	return err
}

// GetConfig is a method of EventReporter
func (e *DatadogStatistics) GetConfig() EventReporterConfig {
	return *e.Config.EventReporterConfig
}

// newReapableEvent is a method of EventReporter
func (e *DatadogStatistics) newReapableEvent(r Reapable, tags []string) error {
	return nil
}

// newBatchReapableEvent is a method of EventReporter
func (e *DatadogStatistics) newBatchReapableEvent(rs []Reapable, tags []string) error {
	return nil
}

// newEvent is a method of EventReporter
func (e *DatadogStatistics) newEvent(string, string, map[string]string, []string) error {
	return nil
}
