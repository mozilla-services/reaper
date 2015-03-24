package aws

import (
	"fmt"
	"net/http"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/ec2"
)

func ExampleMockRoundTrip() {
	creds := aws.Creds("these", "don't", "matter")

	mRound := NewMockRoundTripper()

	// return some testing data from EC2
	// mRound.Handlers is a map[string]MockHandler
	mRound.Handlers["DescribeInstances"] = ReturnXML(ec2.DescribeInstancesResult{
		Reservations: []ec2.Reservation{ec2.Reservation{
			Instances: []ec2.Instance{
				ec2.Instance{InstanceID: aws.String("i-inst1")},
				ec2.Instance{InstanceID: aws.String("i-inst2")},
				ec2.Instance{InstanceID: aws.String("i-inst3")},
			},
		}},
	})

	// use the mockClient instead of http.DefaultClient for transport
	mockClient := &http.Client{Transport: mRound}
	api := ec2.New(creds, "us-west-2", mockClient)

	resp, _ := api.DescribeInstances(&ec2.DescribeInstancesRequest{})

	for _, i := range resp.Reservations[0].Instances {
		fmt.Println(*i.InstanceID)
	}

	// Output:
	// i-inst1
	// i-inst2
	// i-inst3
}
