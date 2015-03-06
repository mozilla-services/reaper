package reaper

import (
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/ec2"
)

type EndpointMap map[string]*ec2.EC2

// AllEndpoints creates a map of initialized *ec2.EC2 endpoints that
// are ready for API calls
func AllEndpoints(creds aws.CredentialsProvider) (EndpointMap, error) {

	e := make(EndpointMap)

	// we just need one in any region
	_api := ec2.New(creds, "us-west-2", nil)
	e["us-west-2"] = _api

	r, err := _api.DescribeRegions(nil)
	if err != nil {
		return nil, err
	}

	for _, region := range r.Regions {
		rName := *region.RegionName
		if e[rName] == nil {
			e[rName] = ec2.New(creds, rName, nil)
		}
	}

	return e, nil
}
