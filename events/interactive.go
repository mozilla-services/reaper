package events

import (
	"bufio"
	"fmt"
	"os"
	"time"

	log "github.com/milescrabill/reaper/reaperlog"
)

type InteractiveEventConfig struct {
	EventReporterConfig
}

type InteractiveEvent struct {
	Config *InteractiveEventConfig
}

func NewInteractiveEvent(c *InteractiveEventConfig) *InteractiveEvent {
	return &InteractiveEvent{
		Config: c,
	}
}

func (n *InteractiveEvent) SetDryRun(b bool) {
	n.Config.DryRun = b
}

func (n *InteractiveEvent) SetNotificationExtras(b bool) {
	n.Config.Extras = b
}

func (n *InteractiveEvent) NewEvent(title string, text string, fields map[string]string, tags []string) error {
	return nil
}
func (n *InteractiveEvent) NewStatistic(name string, value float64, tags []string) error {
	return nil
}
func (n *InteractiveEvent) NewCountStatistic(name string, tags []string) error {
	return nil
}
func (n *InteractiveEvent) NewReapableEvent(r Reapable) error {
	if n.Config.DryRun && n.Config.Extras {
		log.Info(fmt.Sprintf("DryRun: Interactive choice for %s disabled", r.ReapableDescriptionTiny()))
		return nil
	}

	if !n.Config.Triggering(r) && n.Config.Extras {
		log.Notice("Not triggering Interactive Mode for %s", r.ReaperState().State.String())
		return nil
	}

	if r.ReaperState().Until.IsZero() {
		log.Warning("Uninitialized time value for %s!", r.ReapableDescriptionTiny())
	}

	var err error
	if time.Now().After(r.ReaperState().Until) {
		log.Notice(fmt.Sprintf("Choose one of the actions below for %s. All other input is ignored:\nT to terminate\nS to stop\nF to ForceStop\nW to whitelist\nU to update state\n", r.ReapableDescriptionShort()))
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
		case 'U':
			_ = r.IncrementState()
			_, err = r.Save(r.ReaperState())
		}
	}
	return err
}

func (n *InteractiveEvent) NewBatchReapableEvent(rs []Reapable) (errors []error) {
	for _, r := range rs {
		err := n.NewReapableEvent(r)
		if err != nil {
			errors = append(errors, err)
		}
	}
	return
}
