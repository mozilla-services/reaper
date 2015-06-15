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
	conf := Config{
		AWS: reaperaws.AWSConfig{
			HTTP: httpconfig,
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
		HTTP: httpconfig,
		Notifications: reaperevents.NotificationsConfig{
			Extras:             false,
			Interval:           reaperevents.Duration{time.Duration(6) * time.Hour},
			FirstNotification:  reaperevents.Duration{time.Duration(12) * time.Hour},
			SecondNotification: reaperevents.Duration{time.Duration(12) * time.Hour},
			Terminate:          reaperevents.Duration{time.Duration(24) * time.Hour},
		},
		DryRun: true,
	}
	md, err := toml.DecodeFile(path, &conf)
	if err != nil {
		return nil, err
	}

	if len(md.Undecoded()) > 0 {
		log.Error("Undecoded configuration keys: %q\nExiting!", md.Undecoded())
		os.Exit(1)
	}

	conf.SMTP.HTTPConfig = httpconfig

	return &conf, nil
}

// Global reaper config
type Config struct {
	HTTP reaperevents.HTTPConfig

	AWS           reaperaws.AWSConfig
	SMTP          reaperevents.SMTPConfig
	Notifications reaperevents.NotificationsConfig

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
	DataDog reaperevents.DataDogConfig
	Email   reaperevents.SMTPConfig
	Tagger  reaperevents.TaggerConfig
	Reaper  reaperevents.ReaperEventConfig
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
