package main

import (
	"bufio"
	"os"
)

type InteractiveEvent struct{}

func (n *InteractiveEvent) NewEvent(title string, text string, fields map[string]string, tags []string) {
}
func (n *InteractiveEvent) NewStatistic(name string, value float64, tags []string) {}
func (n *InteractiveEvent) NewCountStatistic(name string, tags []string)           {}
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

}
