package events

import log "github.com/mozilla-services/reaper/reaperlog"

// TaggerConfig is the configuration for a Tagger
type TaggerConfig struct {
	EventReporterConfig
}

// Tagger is an EventReporter that tags AWS Resources
type Tagger struct {
	Config *TaggerConfig
}

func (t *Tagger) SetDryRun(b bool) {
	t.Config.DryRun = b
}

func (t *Tagger) SetNotificationExtras(b bool) {
	t.Config.Extras = b
}

func NewTagger(c *TaggerConfig) *Tagger {
	c.Name = "Tagger"
	return &Tagger{c}
}

// Tagger does nothing for most events
func (t *Tagger) NewEvent(title string, text string, fields map[string]string, tags []string) error {
	return nil
}
func (t *Tagger) NewStatistic(name string, value float64, tags []string) error {
	return nil
}
func (t *Tagger) NewCountStatistic(name string, tags []string) error {
	return nil
}
func (t *Tagger) NewReapableEvent(r Reapable) error {
	if r.ReaperState().Until.IsZero() {
		log.Warning("Uninitialized time value for %s!", r.ReapableDescription())
	}

	if t.Config.ShouldTriggerFor(r) {
		log.Notice("Tagging %s with %s", r.ReapableDescriptionTiny(), r.ReaperState().State.String())
		_, err := r.Save(r.ReaperState())
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *Tagger) NewBatchReapableEvent(rs []Reapable) error {
	for _, r := range rs {
		err := e.NewReapableEvent(r)
		if err != nil {
			return err
		}
	}
	return nil
}
