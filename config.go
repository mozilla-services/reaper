package main

import (
	"net/mail"
	"os"
	"time"

	"github.com/BurntSushi/toml"

	reaperaws "github.com/mostlygeek/reaper/aws"
	"github.com/mostlygeek/reaper/events"
	"github.com/mostlygeek/reaper/filters"
)

func LoadConfig(path string) (*Config, error) {
	conf := Config{
		AWS: reaperaws.AWSConfig{},
		SMTP: events.SMTPConfig{
			Host:     "localhost",
			Port:     587,
			AuthType: "none",
			From: events.FromAddress{
				mail.Address{
					Name:    "reaper",
					Address: "aws-reaper@mozilla.com",
				},
			},
		},
		HTTP: events.HTTPConfig{
			TokenSecret: "Default secrets are not safe",
			HTTPApiURL:  "http://localhost",
			HTTPListen:  "localhost:9000",
		},
		Notifications: events.NotificationsConfig{
			Extras:             false,
			Interval:           events.Duration{time.Duration(6) * time.Hour},
			FirstNotification:  events.Duration{time.Duration(12) * time.Hour},
			SecondNotification: events.Duration{time.Duration(12) * time.Hour},
			Terminate:          events.Duration{time.Duration(24) * time.Hour},
		},
		DryRun: true,
	}
	md, err := toml.DecodeFile(path, &conf)
	if err != nil {
		return nil, err
	}

	if len(md.Undecoded()) > 0 {
		Log.Error("Undecoded configuration keys: %q\nExiting!", md.Undecoded())
		os.Exit(1)
	}

	return &conf, nil
}

// Global reaper config
type Config struct {
	HTTP events.HTTPConfig

	AWS           reaperaws.AWSConfig
	SMTP          events.SMTPConfig
	Notifications events.NotificationsConfig

	Events       EventTypes
	Enabled      ResourceTypes
	Filters      FilterTypes
	LogFile      string
	StateFile    string
	WhitelistTag string

	DryRun      bool
	Interactive bool
}

type EventTypes struct {
	DataDog events.DataDogConfig
	Email   events.SMTPConfig
	Tagger  events.TaggerConfig
	Reaper  events.ReaperEventConfig
}

type ResourceTypes struct {
	AutoScalingGroups bool
	Instances         bool
	Snapshots         bool
	Volumes           bool
	SecurityGroups    bool
}

type FilterTypes struct {
	ASG      map[string]filters.Filter
	Instance map[string]filters.Filter
	Snapshot map[string]filters.Filter
}
