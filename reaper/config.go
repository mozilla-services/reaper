package reaper

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"

	reaperaws "github.com/mozilla-services/reaper/aws"
	reaperevents "github.com/mozilla-services/reaper/events"
	"github.com/mozilla-services/reaper/filters"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

func LoadConfig(path string) (*Config, error) {
	httpconfig := reaperevents.HTTPConfig{
		TokenSecret: "Default secrets are not safe",
		APIURL:      "http://localhost",
		Listen:      "localhost:9000",
	}
	notifications := reaperevents.NotificationsConfig{
		StatesConfig: state.StatesConfig{
			Interval:            state.Duration{Duration: time.Duration(6) * time.Hour},
			FirstStateDuration:  state.Duration{Duration: time.Duration(12) * time.Hour},
			SecondStateDuration: state.Duration{Duration: time.Duration(12) * time.Hour},
			ThirdStateDuration:  state.Duration{Duration: time.Duration(12) * time.Hour},
		},
	}
	conf := Config{
		AWS: reaperaws.Config{
			HTTP:          httpconfig,
			Notifications: notifications,
		},
		SMTP: reaperevents.MailerConfig{
			HTTPConfig: httpconfig,
			Host:       "localhost",
			Port:       587,
			AuthType:   "none",
			From: reaperevents.FromAddress{
				Name:    "reaper",
				Address: "aws-reaper@mozilla.com",
			},
		},
		HTTP:          httpconfig,
		Notifications: notifications,
		DryRun:        true,
		Logging: log.LogConfig{
			Extras: true,
		},
	}
	md, err := toml.DecodeFile(path, &conf)
	if err != nil {
		return nil, err
	}

	if len(md.Undecoded()) > 0 {
		log.Error(fmt.Sprintf("Undecoded configuration keys: %q\nExiting!", md.Undecoded()))
		os.Exit(1)
	}

	// set dependent values
	conf.Notifications.StatesConfig = conf.States
	conf.AWS.DryRun = conf.DryRun
	conf.AWS.WhitelistTag = conf.WhitelistTag
	conf.AWS.DefaultOwner = conf.DefaultOwner
	conf.AWS.DefaultEmailHost = conf.DefaultEmailHost
	conf.AWS.Notifications = conf.Notifications
	conf.AWS.HTTP = conf.HTTP
	conf.SMTP.HTTPConfig = conf.HTTP

	log.SetConfig(&conf.Logging)

	// TODO: event reporter dependents are done in reaper.Ready()

	return &conf, nil
}

// Global reaper config
type Config struct {
	HTTP reaperevents.HTTPConfig
	SMTP reaperevents.MailerConfig
	AWS  reaperaws.Config

	Notifications reaperevents.NotificationsConfig
	Logging       log.LogConfig
	States        state.StatesConfig

	Events           EventTypes
	EventTag         string
	LogFile          string
	WhitelistTag     string
	DefaultOwner     string
	DefaultEmailHost string

	AutoScalingGroups ResourceConfig
	Instances         ResourceConfig
	Snapshots         ResourceConfig
	Cloudformations   ResourceConfig
	SecurityGroups    ResourceConfig
	Volumes           ResourceConfig

	DryRun      bool
	Interactive bool
}

type EventTypes struct {
	DatadogEvents     reaperevents.DatadogConfig
	DatadogStatistics reaperevents.DatadogConfig
	Email             reaperevents.MailerConfig
	Tagger            reaperevents.TaggerConfig
	Reaper            reaperevents.ReaperEventConfig
	Interactive       reaperevents.InteractiveEventConfig
}

type ResourceConfig struct {
	Enabled      bool
	FilterGroups map[string]filters.FilterGroup
}
