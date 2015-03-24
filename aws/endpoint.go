package aws

import (
	"net/http"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/ec2"
)

type EndpointMap map[string]*ec2.EC2

// NewEndpoints creates an initialized endpoint map
func NewEndpoints(
	creds aws.CredentialsProvider,
	regions []string,
	client *http.Client) EndpointMap {

	e := make(EndpointMap)

	for _, region := range regions {
		if e[region] == nil {
			e[region] = ec2.New(creds, region, client)
		}
	}

	return e
}
