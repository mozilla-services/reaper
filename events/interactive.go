package events

import (
	"bufio"
	"fmt"
	"os"
)

type InteractiveEventConfig struct {
	Enabled bool
	DryRun  bool
	Extras  bool
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
		log.Info(fmt.Sprintf("DryRun: Interactive choice for %s disabled", r.ReapableDescription()))
		return nil
	}

	if n.Config.Enabled {
		log.Notice(fmt.Sprintf("Choose: T to terminate, S to stop, F to ForceStop, W to whitelist %s. All other input is ignored.", r.ReapableDescription()))
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
		}

		return err
	}
	return nil
}
