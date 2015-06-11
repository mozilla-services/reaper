package events

import (
	"bytes"
	"fmt"
	"net/mail"
	"os"
	"time"

	"github.com/mostlygeek/reaper/reapable"
	"github.com/op/go-logging"
)

var Log *logging.Logger

func init() {
	// set up logging
	Log = logging.MustGetLogger("Reaper")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	format := logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} ▶%{color:reset} %{message}")
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) (err error) {
	d.Duration, err = time.ParseDuration(string(text))
	return
}

type NotificationsConfig struct {
	Extras             bool
	Interval           Duration // like cron, how often to check instances for reaping
	FirstNotification  Duration // how long after start to first notification
	SecondNotification Duration // how long after notify1 to second notification
	Terminate          Duration // how long after notify2 to terminate
}

type Reapable interface {
	reapable.Reapable
	ReapableEventText() *bytes.Buffer
	//ReapableEventHTML() *bytes.Buffer
	ReapableEventEmail() (mail.Address, string, string, error)
}

type EventReporter interface {
	NewEvent(title string, text string, fields map[string]string, tags []string) error
	NewStatistic(name string, value float64, tags []string) error
	NewCountStatistic(name string, tags []string) error
	NewReapableEvent(r Reapable) error
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

func (n *NoEventReporter) NewReapableEvent(r Reapable) error {
	return nil
}

type ErrorEventReporter struct{}

func (e *ErrorEventReporter) NewEvent(title string, text string, fields map[string]string, tags []string) error {
	return fmt.Errorf("ErrorEventReporter")
}

func (e *ErrorEventReporter) NewStatistic(name string, value float64, tags []string) error {
	return fmt.Errorf("ErrorEventReporter")
}

func (e *ErrorEventReporter) NewCountStatistic(name string, tags []string) error {
	return fmt.Errorf("ErrorEventReporter")
}

func (e *ErrorEventReporter) NewReapableEvent(r Reapable) error {
	return fmt.Errorf("ErrorEventReporter")
}
