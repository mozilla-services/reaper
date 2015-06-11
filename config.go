package main

import (
	"net/mail"
	"os"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/mostlygeek/reaper/events"
)

func LoadConfig(path string) (*Config, error) {
	conf := Config{
		AWS: AWSConfig{},
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
		Reaper: ReaperConfig{
			Interval:           duration{time.Duration(6) * time.Hour},
			FirstNotification:  duration{time.Duration(12) * time.Hour},
			SecondNotification: duration{time.Duration(12) * time.Hour},
			Terminate:          duration{time.Duration(24) * time.Hour},
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

	AWS    AWSConfig
	SMTP   events.SMTPConfig
	Reaper ReaperConfig

	Events        EventTypes
	Notifications NotificationTypes
	Enabled       ResourceTypes
	Filters       FilterTypes
	LogFile       string
	StateFile     string
	WhitelistTag  string

	DryRun      bool
	Interactive bool
}

type NotificationTypes struct {
	Extras bool
}

type EventTypes struct {
	DataDog events.DataDogConfig
	Email   events.SMTPConfig
	Tagger  TaggerConfig
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
	ASG      map[string]Filter
	Instance map[string]Filter
	Snapshot map[string]Filter
}

type Filter struct {
	Function  string
	Arguments []string
}

func (filter *Filter) Int64Value(v int) (int64, error) {
	// parseint -> base 10, 64 bit int
	i, err := strconv.ParseInt(filter.Arguments[v], 10, 64)
	if err != nil {
		Log.Error("could not parse %s as int64", filter.Arguments[v])
		return 0, err
	}
	return i, nil
}

func (filter *Filter) BoolValue(v int) (bool, error) {
	b, err := strconv.ParseBool(filter.Arguments[v])
	if err != nil {
		Log.Error("could not parse %s as bool", filter.Arguments[v])
		return false, err
	}
	return b, nil
}

type AWSConfig struct {
	Regions []string
}

// controls behaviour of the EC2 single instance reaper works
type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) (err error) {
	d.Duration, err = time.ParseDuration(string(text))
	return
}

type ReaperConfig struct {
	Interval           duration // like cron, how often to check instances for reaping
	FirstNotification  duration // how long after start to first notification
	SecondNotification duration // how long after notify1 to second notification
	Terminate          duration // how long after notify2 to terminate
}
