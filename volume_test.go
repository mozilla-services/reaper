package main_test

import (
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/service/ec2"
	"github.com/mostlygeek/reaper"
	"testing"
)

// this is an awful test
func TestNewVolume(t *testing.T) {
	ec2 := ec2.New(&aws.Config{Region: TESTREGION})
	resp, err := ec2.DescribeVolumes(nil)

	if err != nil {
		// do something
	}

	s := reaper.NewVolume(TESTREGION, resp.Volumes[0])

	if s.Id() == "" {
		t.Error("volume improperly initialized")
	}
}
