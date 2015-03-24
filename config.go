package reaper

import (
	"fmt"
	"net/smtp"
	"time"

	"github.com/BurntSushi/toml"
)

func LoadConfig(path string) (*Config, error) {

	conf := Config{
		AWS: AWSConfig{
			Regions: []string{
				"us-west-1",
				"us-west-2",
				"us-east-1",
				"eu-west-1",
				"eu-central-1",
				"ap-southeast-1",
				"ap-southeast-2",
				"ap-northeast-1",
				"sa-east-1",
			},
		},
		SMTP: SMTPConfig{
			Address:  "localhost",
			Port:     587,
			AuthType: "none",
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
	AWS    AWSConfig
	SMTP   SMTPConfig
	Reaper ReaperConfig
}

type SMTPConfig struct {
	Address  string // must include port
	Port     int
	AuthType string
	Username string
	Password string
}

func (s *SMTPConfig) String() string {
	return fmt.Sprintf("%s:%d auth type:%s, creds: %s:%s",
		s.Address,
		s.Port,
		s.AuthType,
		s.Username,
		s.Password)
}
func (s *SMTPConfig) Addr() string {
	if s.Port == 0 {
		// friends don't let friend's smtp over port 25
		return fmt.Sprintf("%s:%s", s.Address, 587)
	} else {
		return fmt.Sprintf("%s:%d", s.Address, s.Port)
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
	return smtp.PlainAuth("", s.Username, s.Password, s.Address)
}

type AWSConfig struct {
	AccessID     string
	AccessSecret string
	Token        string
	Regions      []string
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
