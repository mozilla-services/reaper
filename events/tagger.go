package events

import (
	"time"

	log "github.com/mostlygeek/reaper/reaperlog"
)

// TaggerConfig is the configuration for a Tagger
type TaggerConfig struct {
	Enabled bool
	DryRun  bool
	Extras  bool
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
	return &Tagger{
		Config: c,
	}
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
	if t.Config.DryRun && t.Config.Extras {
		log.Notice("DryRun: Not tagging %s", r.ReapableDescription())
		return nil
	}

	if time.Now().After(r.ReaperState().Until) {
		r.IncrementState()
	}
	_, err := r.Save(r.ReaperState())
	if err != nil {
		return err
	}
	return nil
}
