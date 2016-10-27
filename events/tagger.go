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

// setDryRun is a method of EventReporter
func (e *Tagger) setDryRun(b bool) {
	e.Config.DryRun = b
}

// NewTagger returns a new instance of Tagger
func NewTagger(c *TaggerConfig) *Tagger {
	c.Name = "Tagger"
	return &Tagger{c}
}

// newReapableEvent is a method of EventReporter
func (e *Tagger) newReapableEvent(r Reapable, tags []string) error {
	if r.ReaperState().Until.IsZero() {
		log.Warning("Uninitialized time value for %s!", r.ReapableDescription())
	}

	if e.Config.shouldTriggerFor(r) {
		if log.Extras() {
			log.Info("%s: Tagging %s with %s", e.Config.Name, r.ReapableDescriptionTiny(), r.ReaperState().State.String())
		}
		if e.Config.DryRun {
			return nil
		}
		_, err := r.Save(r.ReaperState())
		if err != nil {
			return err
		}
	}
	return nil
}

// newBatchReapableEvent is a method of EventReporter
func (e *Tagger) newBatchReapableEvent(rs []Reapable, tags []string) error {
	for _, r := range rs {
		err := e.newReapableEvent(r, tags)
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

// newCountStatistic is a method of EventReporter
func (e *Tagger) newCountStatistic(string, []string) error {
	return nil
}

// newStatistic is a method of EventReporter
func (e *Tagger) newStatistic(string, float64, []string) error {
	return nil
}

// newEvent is a method of EventReporter
func (e *Tagger) newEvent(string, string, map[string]string, []string) error {
	return nil
}
