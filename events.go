package main

type EventReporter interface {
	NewEvent(title string, text string, fields map[string]string, tags []string)
	NewStatistic(name string, value float64, tags []string)
	NewCountStatistic(name string, tags []string)
	NewReapableInstanceEvent(i *Instance)
	NewReapableASGEvent(a *AutoScalingGroup)
}

// implements EventReporter but does nothing
type NoEventReporter struct{}

func (n *NoEventReporter) NewEvent(title string, text string, fields map[string]string, tags []string) {
}
func (n *NoEventReporter) NewStatistic(name string, value float64, tags []string) {}
func (n *NoEventReporter) NewCountStatistic(name string, tags []string)           {}
func (n *NoEventReporter) NewReapableInstanceEvent(i *Instance)                   {}
func (n *NoEventReporter) NewReapableASGEvent(a *AutoScalingGroup)                {}

type InstanceEventData struct {
	Config   *Config
	Instance *Instance
}

type ASGEventData struct {
	Config *Config
	ASG    *AutoScalingGroup
}
