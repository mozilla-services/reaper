package main_test

import (
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/service/ec2"
	"github.com/mostlygeek/reaper"
	"testing"
)

// this is an awful test
func TestNewSecurityGroup(t *testing.T) {
	ec2 := ec2.New(&aws.Config{Region: TESTREGION})
	resp, err := ec2.DescribeSecurityGroups(nil)

	if err != nil {
		// do something
	}

	s := reaper.NewSecurityGroup(TESTREGION, resp.SecurityGroups[0])

	if s.Id() == "" {
		t.Error("security group improperly initialized")
	}
}
