package aws

import "github.com/mostlygeek/reaper/events"

var config *AWSConfig

type AWSConfig struct {
	Notifications events.NotificationsConfig
	HTTP          events.HTTPConfig
	Regions       []string
	WhitelistTag  string
}

func NewAWSConfig() *AWSConfig {
	return &AWSConfig{}
}

func SetAWSConfig(c *AWSConfig) {
	config = c
}
