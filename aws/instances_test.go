package aws

import (
	"net/http"
	"testing"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/ec2"
)

func TestParseState(t *testing.T) {
	in := (&State{}).String()

	if ParseState(in).String() != in {
		t.Error()
	}
}

func TestParseInvalid(t *testing.T) {
	expected := (&State{}).String() // the default values

	// should all be unparseable
	a := []string{
		"start|2015-01-24 25PM MST",
		"delay|2015-01-24 19PM",
	}

	for _, test := range a {
		s := ParseState(test)
		if s.String() != expected {
			t.Errorf("Failed on: %s", test)
		}
	}
}

func TestAllInstances(t *testing.T) {

	creds := aws.Creds("these", "don't", "matter")

	mRound := NewMockRoundTripper()
	mockClient := &http.Client{Transport: mRound}

	mRound.Handlers["DescribeInstances"] = func(req *http.Request) (*http.Response, error) {
		// figure out which endpoint is being asked and generate some mock data
		// for each specific instance
		state := &ec2.InstanceState{Name: aws.String("running")}
		switch req.URL.Host {
		case "ec2.us-west-1.amazonaws.com":
			return ReturnXML(ec2.DescribeInstancesResult{
				Reservations: []ec2.Reservation{ec2.Reservation{
					Instances: []ec2.Instance{
						ec2.Instance{InstanceID: aws.String("i-west1a"), State: state},
					},
				}},
			})(req) // <-- notice we create and execute a new handler
		case "ec2.us-west-2.amazonaws.com":
			return ReturnXML(ec2.DescribeInstancesResult{
				Reservations: []ec2.Reservation{ec2.Reservation{
					Instances: []ec2.Instance{
						ec2.Instance{InstanceID: aws.String("i-west2a"), State: state},
					},
				}},
			})(req)
		default:
			return MockNotImplemented(req), nil
		}
	}

	regions := []string{
		"us-west-1",
		"us-west-2",
	}

	endpoints := NewEndpoints(creds, regions, mockClient)
	instances := AllInstances(endpoints)

	if len(instances) != 2 {
		t.Errorf("Expected 2 instances, got: %d", len(instances))
	}
}
