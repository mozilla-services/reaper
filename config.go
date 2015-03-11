package reaper

import (
	"fmt"
	"net/smtp"
	"time"

	"github.com/BurntSushi/toml"
)

func LoadConfig(path string) (*Config, error) {

	conf := Config{
		Credentials: CredConfig{},
		SMTP: SMTPConfig{
			Address:  "localhost",
			Port:     587,
			AuthType: "none",
		},
		Instances: InstanceConfig{
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
	Credentials CredConfig
	SMTP        SMTPConfig
	Instances   InstanceConfig
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
	return smtp.PlainAuth("", s.Username, s.Password, s.Addr())
}

type CredConfig struct {
	AccessID     string
	AccessSecret string
	Token        string
}

// controls behaviour of the EC2 single instance reaper works
type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) (err error) {
	d.Duration, err = time.ParseDuration(string(text))
	return
}

type InstanceConfig struct {
	Interval           duration
	FirstNotification  duration
	SecondNotification duration
	Terminate          duration
}
