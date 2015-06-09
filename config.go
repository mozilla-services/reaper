package main

import (
	"fmt"
	"net/mail"
	"net/smtp"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
)

func LoadConfig(path string) (*Config, error) {
	conf := Config{
		TokenSecret: "Default secrets are not safe",
		HTTPApiURL:  "http://localhost",
		HTTPListen:  "localhost:9000",
		AWS:         AWSConfig{},
		SMTP: SMTPConfig{
			Host:     "localhost",
			Port:     587,
			AuthType: "none",
			From: FromAddress{
				mail.Address{
					Name:    "reaper",
					Address: "aws-reaper@mozilla.com",
				},
			},
		},
		Reaper: ReaperConfig{
			Interval:           duration{time.Duration(6) * time.Hour},
			FirstNotification:  duration{time.Duration(12) * time.Hour},
			SecondNotification: duration{time.Duration(12) * time.Hour},
			Terminate:          duration{time.Duration(24) * time.Hour},
		},
		DryRun: true,
	}

	if _, err := toml.DecodeFile(path, &conf); err != nil {
		return nil, err
	}

	return &conf, nil
}

// Global reaper config
type Config struct {
	HTTPApiURL  string // used to generate the link
	HTTPListen  string // host:port the web server will attempt to listen on
	TokenSecret string // used for token generation

	AWS    AWSConfig
	SMTP   SMTPConfig
	Reaper ReaperConfig

	Events        EventTypes
	Notifications NotificationTypes
	Enabled       ResourceTypes
	Filters       FilterTypes
	LogFile       string
	StateFile     string

	DryRun      bool
	Interactive bool
}

type NotificationTypes struct {
	Extras bool
}

type EventTypes struct {
	DataDog DataDogConfig
	Email   SMTPConfig
	Tagger  TaggerConfig
	Reaper  ReaperEventConfig
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

type FromAddress struct {
	mail.Address
}

func (f *FromAddress) UnmarshalText(text []byte) error {
	a, err := mail.ParseAddress(string(text))
	if err != nil {
		return err
	}

	f.Address = *a
	return nil
}

type SMTPConfig struct {
	Enabled  bool
	Host     string
	Port     int
	AuthType string
	Username string
	Password string
	From     FromAddress
}

func (s *SMTPConfig) String() string {
	return fmt.Sprintf("%s:%d auth type:%s, creds: %s:%s",
		s.Host,
		s.Port,
		s.AuthType,
		s.Username,
		s.Password)
}
func (s *SMTPConfig) Addr() string {
	if s.Port == 0 {
		// friends don't let friend's smtp over port 25
		return fmt.Sprintf("%s:%d", s.Host, 587)
	}
	// default
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// Auth creates the appropriate smtp.Auth from the configured AuthType
func (s *SMTPConfig) Auth() smtp.Auth {
	switch s.AuthType {
	case "md5":
		return s.CRAMMD5Auth()
	case "plain":
		return s.PlainAuth()
	default:
		return nil
	}
}

func (s *SMTPConfig) CRAMMD5Auth() smtp.Auth {
	return smtp.CRAMMD5Auth(s.Username, s.Password)
}

func (s *SMTPConfig) PlainAuth() smtp.Auth {
	return smtp.PlainAuth("", s.Username, s.Password, s.Host)
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
