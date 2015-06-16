package aws

import "github.com/milescrabill/reaper/events"

var config *AWSConfig

type AWSConfig struct {
	Notifications events.NotificationsConfig
	HTTP          events.HTTPConfig
	Regions       []string
	WhitelistTag  string
	DefaultOwner  string
	DryRun        bool
}

func NewAWSConfig() *AWSConfig {
	return &AWSConfig{}
}

func SetAWSConfig(c *AWSConfig) {
	config = c
}
