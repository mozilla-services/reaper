package reaper

import (
	"net/mail"
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
		ApiURL:      "http://localhost",
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
		AWS: reaperaws.AWSConfig{
			HTTP:          httpconfig,
			Notifications: notifications,
		},
		SMTP: reaperevents.SMTPConfig{
			HTTPConfig: httpconfig,
			Host:       "localhost",
			Port:       587,
			AuthType:   "none",
			From: reaperevents.FromAddress{
				Address: mail.Address{
					Name:    "reaper",
					Address: "aws-reaper@mozilla.com",
				},
			},
		},
		HTTP:          httpconfig,
		Notifications: notifications,
		DryRun:        true,
		Logging: log.LogConfig{
			Extras: true,
		},
		DefaultEmailHost: "mozilla.com",
	}
	md, err := toml.DecodeFile(path, &conf)
	if err != nil {
		return nil, err
	}

	if len(md.Undecoded()) > 0 {
		log.Error("Undecoded configuration keys: %q\nExiting!", md.Undecoded())
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

	// TODO: event reporter dependents are done in reaper.Ready()

	return &conf, nil
}

// Global reaper config
type Config struct {
	HTTP reaperevents.HTTPConfig

	AWS           reaperaws.AWSConfig
	SMTP          reaperevents.SMTPConfig
	Notifications reaperevents.NotificationsConfig
	Logging       log.LogConfig
	States        state.StatesConfig

	Events           EventTypes
	LogFile          string
	StateFile        string
	WhitelistTag     string
	DefaultOwner     string
	DefaultEmailHost string

	AutoScalingGroups FilterGroup
	Instances         FilterGroup
	Snapshots         FilterGroup

	DryRun            bool
	Interactive       bool
	LoadFromStateFile bool
}

type EventTypes struct {
	DataDog     reaperevents.DataDogConfig
	Email       reaperevents.SMTPConfig
	Tagger      reaperevents.TaggerConfig
	Reaper      reaperevents.ReaperEventConfig
	Interactive reaperevents.InteractiveEventConfig
}

type FilterGroup struct {
	Enabled bool
	Filters map[string]filters.Filter
}
