package events

import "fmt"

type EventReporter interface {
	NewEvent(title string, text string, fields map[string]string, tags []string) error
	NewStatistic(name string, value float64, tags []string) error
	NewCountStatistic(name string, tags []string) error
}

// implements EventReporter but does nothing
type NoEventReporter struct{}

func (n *NoEventReporter) NewEvent(title string, text string, fields map[string]string, tags []string) error {
	return nil
}
func (n *NoEventReporter) NewStatistic(name string, value float64, tags []string) error {
	return nil
}
func (n *NoEventReporter) NewCountStatistic(name string, tags []string) error {
	return nil
}

type ErrorEventReporter struct{}

func (r *ErrorEventReporter) NewEvent(title string, text string, fields map[string]string, tags []string) error {
	return fmt.Errorf("ErrorEventReporter")
}

func (r *ErrorEventReporter) NewStatistic(name string, value float64, tags []string) error {
	return fmt.Errorf("ErrorEventReporter")
}

func (r *ErrorEventReporter) NewCountStatistic(name string, tags []string) error {
	return fmt.Errorf("ErrorEventReporter")
}
