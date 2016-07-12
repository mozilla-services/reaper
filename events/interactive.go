package events

import (
	"bufio"
	"os"

	log "github.com/mozilla-services/reaper/reaperlog"
)

// InteractiveEventConfig is the configuration for an InteractiveEvent
type InteractiveEventConfig struct {
	*eventReporterConfig
}

// InteractiveEvent implements ReapableEventReporter, offers choices
// uses godspeed, requires dd-agent running
type InteractiveEvent struct {
	Config *InteractiveEventConfig
}

// NewInteractiveEvent returns a new instance of InteractiveEvent
func NewInteractiveEvent(c *InteractiveEventConfig) *InteractiveEvent {
	c.Name = "InteractiveEvent"
	return &InteractiveEvent{c}
}

// SetDryRun is a method of ReapableEventReporter
func (e *InteractiveEvent) SetDryRun(b bool) {
	e.Config.DryRun = b
}

// NewReapableEvent is a method of ReapableEventReporter
func (e *InteractiveEvent) NewReapableEvent(r Reapable, tags []string) error {
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

// NewBatchReapableEvent is a method of EventReporter
func (e *InteractiveEvent) NewBatchReapableEvent(rs []Reapable, tags []string) error {
	for _, r := range rs {
		err := e.NewReapableEvent(r, tags)
		if err != nil {
			return err
		}
	}
	return nil
}
