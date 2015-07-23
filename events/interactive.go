package events

import (
	"bufio"
	"fmt"
	"os"

	log "github.com/mozilla-services/reaper/reaperlog"
)

type InteractiveEventConfig struct {
	EventReporterConfig
}

type InteractiveEvent struct {
	Config *InteractiveEventConfig
}

func NewInteractiveEvent(c *InteractiveEventConfig) *InteractiveEvent {
	c.Name = "InteractiveEvent"
	return &InteractiveEvent{c}
}

func (n *InteractiveEvent) SetDryRun(b bool) {
	n.Config.DryRun = b
}

func (*InteractiveEvent) Cleanup() error { return nil }

func (*InteractiveEvent) NewEvent(title string, text string, fields map[string]string, tags []string) error {
	return nil
}
func (*InteractiveEvent) NewStatistic(name string, value float64, tags []string) error {
	return nil
}
func (*InteractiveEvent) NewCountStatistic(name string, tags []string) error {
	return nil
}
func (n *InteractiveEvent) NewReapableEvent(r Reapable, tags []string) error {
	if r.ReaperState().Until.IsZero() {
		log.Warning("Uninitialized time value for %s!", r.ReapableDescriptionTiny())
	}

	var err error
	if n.Config.ShouldTriggerFor(r) {
		log.Notice(fmt.Sprintf("Choose one of the actions below for %s. All other input is ignored:\nT to terminate\nS to stop\nF to ForceStop\nW to whitelist\nI to increment state\nU to unsave state\n", r.ReapableDescriptionShort()))
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Error(fmt.Sprintf("%s", err.Error()))
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

func (e *InteractiveEvent) NewBatchReapableEvent(rs []Reapable, tags []string) error {
	for _, r := range rs {
		err := e.NewReapableEvent(r, tags)
		if err != nil {
			return err
		}
	}
	return nil
}
