package events

import "time"

// TaggerConfig is the configuration for a Tagger
type TaggerConfig struct {
	Enabled bool
}

// Tagger is an EventReporter that tags AWS Resources
type Tagger struct {
	Config *TaggerConfig
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
	if time.Now().After(r.ReaperState().Until) {
		r.IncrementState()
	}
	_, err := r.Save(r.ReaperState())
	if err != nil {
		return err
	}
	return nil
}
