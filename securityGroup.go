package main

import (
	"github.com/aws/aws-sdk-go/service/ec2"
)

type SecurityGroups []*SecurityGroup
type SecurityGroup struct {
	AWSResource
}

func NewSecurityGroup(region string, sg *ec2.SecurityGroup) *SecurityGroup {
	s := SecurityGroup{
		AWSResource{
			ID:          *sg.GroupID,
			Name:        *sg.GroupName,
			Region:      region,
			Description: *sg.Description,
			VPCID:       *sg.VPCID,
			OwnerID:     *sg.OwnerID,
			Tags:        make(map[string]string),
		},
	}

	for _, tag := range sg.Tags {
		s.Tags[*tag.Key] = *tag.Value
	}

	s.reaperState = ParseState(s.Tags[reaperTag])

	return &s
}
