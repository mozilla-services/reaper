package events

import (
	"bufio"
	"os"

	log "github.com/mozilla-services/reaper/reaperlog"
)

// InteractiveEventConfig is the configuration for an InteractiveEvent
type InteractiveEventConfig struct {
	*EventReporterConfig
}

// InteractiveEvent implements EventReporter, offers choices
// uses godspeed, requires dd-agent running
type InteractiveEvent struct {
	Config *InteractiveEventConfig
}

// NewInteractiveEvent returns a new instance of InteractiveEvent
func NewInteractiveEvent(c *InteractiveEventConfig) *InteractiveEvent {
	c.Name = "InteractiveEvent"
	return &InteractiveEvent{c}
}

// setDryRun is a method of EventReporter
func (e *InteractiveEvent) setDryRun(b bool) {
	e.Config.DryRun = b
}

// newReapableEvent is a method of EventReporter
func (e *InteractiveEvent) newReapableEvent(r Reapable, tags []string) error {
	if r.ReaperState().Until.IsZero() {
		log.Warning("Uninitialized time value for %s!", r.ReapableDescriptionTiny())
	}

	var err error
	if e.Config.shouldTriggerFor(r) {
		log.Info("Choose one of the actions below for %s. All other input is ignored:\nT to terminate\nS to stop\nF to ForceStop\nW to whitelist\nI to increment state\nU to unsave state\n", r.ReapableDescriptionShort())
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Error(err.Error())
		}
		inputChar := input[0]
		switch inputChar {
		// maybe todo: use the ok value
		case 'T':
			_, err = r.Terminate()
		case 'S':
			_, err = r.Stop()
		case 'W':
			_, err = r.Whitelist()
		case 'F':
			_, err = r.ForceStop()
		case 'I':
			_ = r.IncrementState()
			_, err = r.Save(r.ReaperState())
		case 'U':
			_, err = r.Unsave()
		}
	}
	return err
}

// newBatchReapableEvent is a method of EventReporter
func (e *InteractiveEvent) newBatchReapableEvent(rs []Reapable, tags []string) error {
	for _, r := range rs {
		err := e.newReapableEvent(r, tags)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetConfig is a method of EventReporter
func (e *InteractiveEvent) GetConfig() EventReporterConfig {
	return *e.Config.EventReporterConfig
}

// newCountStatistic is a method of EventReporter
func (e *InteractiveEvent) newCountStatistic(string, []string) error {
	return nil
}

// newStatistic is a method of EventReporter
func (e *InteractiveEvent) newStatistic(string, float64, []string) error {
	return nil
}

// newEvent is a method of EventReporter
func (e *InteractiveEvent) newEvent(string, string, map[string]string, []string) error {
	return nil
}
