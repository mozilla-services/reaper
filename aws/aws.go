package aws

import (
	"os"

	"github.com/mostlygeek/reaper/events"
	"github.com/op/go-logging"
)

var Config AWSConfig
var Log *logging.Logger

type AWSConfig struct {
	Notifications events.NotificationsConfig
	HTTP          events.HTTPConfig
	Regions       []string
	WhitelistTag  string
}

func init() {
	// set up logging
	Log = logging.MustGetLogger("Reaper")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	format := logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} ▶%{color:reset} %{message}")
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)
}

func NewAWSConfig() *AWSConfig {
	return &AWSConfig{}
}

func SetNotificationsConfig(c *events.NotificationsConfig) {
	Config.Notifications = *c
}

func SetHTTPConfig(c *events.HTTPConfig) {
	Config.HTTP = *c
}

func SetRegions(regions []string) {
	Config.Regions = regions
}
