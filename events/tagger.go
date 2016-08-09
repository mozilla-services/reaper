package events

import log "github.com/mozilla-services/reaper/reaperlog"

// TaggerConfig is the configuration for a Tagger
type TaggerConfig struct {
	*EventReporterConfig
}

// Tagger is an EventReporter that tags AWS Resources
type Tagger struct {
	Config *TaggerConfig
}

// SetDryRun is a method of EventReporter
func (e *Tagger) SetDryRun(b bool) {
	e.Config.DryRun = b
}

// NewTagger returns a new instance of Tagger
func NewTagger(c *TaggerConfig) *Tagger {
	c.Name = "Tagger"
	return &Tagger{c}
}

// NewReapableEvent is a method of EventReporter
func (e *Tagger) NewReapableEvent(r Reapable, tags []string) error {
	if r.ReaperState().Until.IsZero() {
		log.Warning("Uninitialized time value for %s!", r.ReapableDescription())
	}

	if e.Config.shouldTriggerFor(r) {
		log.Info("Tagging %s with %s", r.ReapableDescriptionTiny(), r.ReaperState().State.String())
		_, err := r.Save(r.ReaperState())
		if err != nil {
			return err
		}
	}
	return nil
}

// NewBatchReapableEvent is a method of EventReporter
func (e *Tagger) NewBatchReapableEvent(rs []Reapable, tags []string) error {
	for _, r := range rs {
		err := e.NewReapableEvent(r, tags)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetConfig is a method of EventReporter
func (e *Tagger) GetConfig() EventReporterConfig {
	return *e.Config.EventReporterConfig
}

// NewCountStatistic is a method of EventReporter
func (e *Tagger) NewCountStatistic(string, []string) error {
	return nil
}

// NewStatistic is a method of EventReporter
func (e *Tagger) NewStatistic(string, float64, []string) error {
	return nil
}

// NewEvent is a method of EventReporter
func (e *Tagger) NewEvent(string, string, map[string]string, []string) error {
	return nil
}
