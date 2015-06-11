package main

import (
	"bufio"
	"os"

	"github.com/mostlygeek/reaper/events"
)

type InteractiveEvent struct{}

func (n *InteractiveEvent) NewEvent(title string, text string, fields map[string]string, tags []string) error {
	return nil
}
func (n *InteractiveEvent) NewStatistic(name string, value float64, tags []string) error {
	return nil
}
func (n *InteractiveEvent) NewCountStatistic(name string, tags []string) error {
	return nil
}
func (n *InteractiveEvent) NewReapableEvent(r events.Reapable) error {
	return nil
}

func (n *InteractiveEvent) NewReapableInstanceEvent(i *Instance) {
	Log.Notice("Press T to terminate, S to stop, W to whitelist %s in region %s. All other keys are ignored.", i.ID, i.Region)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		Log.Error("%s", err.Error())
	}

	inputChar := input[0]

	if inputChar == 'T' {
		Log.Debug("Terminating %s in region %s", i.ID, i.Region)
		i.Terminate()
	} else if inputChar == 'S' {
		Log.Debug("Stopping %s in region %s", i.ID, i.Region)
		i.Stop()
	} else if inputChar == 'W' {
		Log.Debug("Whitelisting %s in region %s", i.ID, i.Region)
		i.Whitelist()
	}
}
func (n *InteractiveEvent) NewReapableASGEvent(a *AutoScalingGroup) {
	Log.Notice("Press T to terminate, S to stop, W to whitelist %s in region %s. All other keys are ignored.", a.ID, a.Region)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		Log.Error("%s", err.Error())
	}

	inputChar := input[0]

	if inputChar == 'T' {
		Log.Debug("Terminating %s in region %s", a.ID, a.Region)
		a.Terminate()
	} else if inputChar == 'S' {
		Log.Debug("Stopping %s in region %s", a.ID, a.Region)
		a.Stop()
	} else if inputChar == 'W' {
		Log.Debug("Whitelisting %s in region %s", a.ID, a.Region)
		a.Whitelist()
	}
}
