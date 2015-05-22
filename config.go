package reaper

import (
	"fmt"
	"net/mail"
	"net/smtp"
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
			From:     FromAddress{mail.Address{"reaper", "aws-reaper@mozilla.com"}},
		},
		Reaper: ReaperConfig{
			Interval:           duration{time.Duration(6) * time.Hour},
			FirstNotification:  duration{time.Duration(12) * time.Hour},
			SecondNotification: duration{time.Duration(12) * time.Hour},
			Terminate:          duration{time.Duration(24) * time.Hour},
		},
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
		return fmt.Sprintf("%s:%s", s.Host, 587)
	} else {
		return fmt.Sprintf("%s:%d", s.Host, s.Port)
	}
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

type Filter struct {
	Name   string
	Values []string
}

type AWSConfig struct {
	Regions []string
	Filters map[string]Filter
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
