package reaper

import (
	"net/mail"
	"os"
	"time"

	"github.com/BurntSushi/toml"

	reaperaws "github.com/milescrabill/reaper/aws"
	reaperevents "github.com/milescrabill/reaper/events"
	"github.com/milescrabill/reaper/filters"
	log "github.com/milescrabill/reaper/reaperlog"
)

func LoadConfig(path string) (*Config, error) {
	httpconfig := reaperevents.HTTPConfig{
		TokenSecret: "Default secrets are not safe",
		ApiURL:      "http://localhost",
		Listen:      "localhost:9000",
	}
	notifications := reaperevents.NotificationsConfig{
		Extras:             true,
		Interval:           reaperevents.Duration{time.Duration(6) * time.Hour},
		FirstNotification:  reaperevents.Duration{time.Duration(12) * time.Hour},
		SecondNotification: reaperevents.Duration{time.Duration(12) * time.Hour},
		Terminate:          reaperevents.Duration{time.Duration(24) * time.Hour},
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
				mail.Address{
					Name:    "reaper",
					Address: "aws-reaper@mozilla.com",
				},
			},
		},
		HTTP:          httpconfig,
		Notifications: notifications,
		DryRun:        true,
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
	conf.AWS.DryRun = conf.DryRun
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

	Events       EventTypes
	LogFile      string
	StateFile    string
	WhitelistTag string

	AutoScalingGroups FilterGroup
	Instances         FilterGroup
	Snapshots         FilterGroup

	DryRun      bool
	Interactive bool
}

type EventTypes struct {
	DataDog reaperevents.DataDogConfig
	Email   reaperevents.SMTPConfig
	Tagger  reaperevents.TaggerConfig
	Reaper  reaperevents.ReaperEventConfig
}

type FilterGroup struct {
	Enabled bool
	Filters map[string]filters.Filter
}
