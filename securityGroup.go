package reaper

import (
	"github.com/awslabs/aws-sdk-go/service/ec2"
)

type SecurityGroups []*SecurityGroup
type SecurityGroup struct {
	AWSResource
}

func NewSecurityGroup(region string, sg *ec2.SecurityGroup) *SecurityGroup {
	s := SecurityGroup{
		AWSResource{
			id:          *sg.GroupID,
			name:        *sg.GroupName,
			region:      region,
			description: *sg.Description,
			vpc_id:      *sg.VPCID,
			owner_id:    *sg.OwnerID,
			tags:        make(map[string]string),
		},
	}

	for _, tag := range sg.Tags {
		s.tags[*tag.Key] = *tag.Value
	}

	s.reaper = ParseState(s.tags[reaper_tag])

	return &s
}
