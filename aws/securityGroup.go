package aws

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/milescrabill/reaper/reapable"
	"github.com/milescrabill/reaper/state"
)

type SecurityGroups []*SecurityGroup
type SecurityGroup struct {
	AWSResource
}

func NewSecurityGroup(region string, sg *ec2.SecurityGroup) *SecurityGroup {
	s := SecurityGroup{
		AWSResource{
			ID:          reapable.ID(*sg.GroupID),
			Name:        *sg.GroupName,
			Region:      reapable.Region(region),
			Description: *sg.Description,
			VPCID:       *sg.VPCID,
			OwnerID:     *sg.OwnerID,
			Tags:        make(map[string]string),
		},
	}

	for _, tag := range sg.Tags {
		s.Tags[*tag.Key] = *tag.Value
	}

	s.reaperState = state.NewStateWithTag(s.Tags[reaperTag])

	return &s
}
