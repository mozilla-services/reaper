package reaper

import (
	"github.com/BurntSushi/toml"
	"time"
)

func LoadConfig(path string) (*Config, error) {

	var config Config
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// Global reaper config
type Config struct {
	Credentials CredConfig
	Instances   InstanceConfig
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
