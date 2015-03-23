package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/ec2"
)

type MockHandler func(*http.Request) (*http.Response, error)

type MockRoundTripper struct {
	Handlers map[string]MockHandler
}

func NewMockRoundTripper() *MockRoundTripper {
	return &MockRoundTripper{Handlers: make(map[string]MockHandler)}
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {

	// parse out the form data..

	data, _ := ioutil.ReadAll(req.Body)
	values, _ := url.ParseQuery(string(data))

	// replace the body data so other things can read it
	req.Body = ioutil.NopCloser(bytes.NewReader(data))

	action := ""
	if _, found := values["Action"]; found && len(values["Action"]) > 0 {
		action = values["Action"][0]
	}

	fn, ok := m.Handlers[action]

	if ok != true {
		x := ec2ErrorResponse{
			XMLName:   xml.Name{"Space", "Local"},
			Type:      "Not Implemented",
			Code:      "501",
			Message:   "No Mock Implementation for action: " + action,
			RequestID: "000",
		}

		bodyR, err := xml.Marshal(x)
		if err != nil {
			return nil, err
		}

		resp := &http.Response{
			Status:     http.StatusText(http.StatusNotImplemented),
			StatusCode: http.StatusNotImplemented,
			Proto:      req.Proto,
			ProtoMajor: req.ProtoMajor,
			ProtoMinor: req.ProtoMinor,
			Body:       ioutil.NopCloser(bytes.NewReader(bodyR)),
		}

		return resp, nil
	} else {
		return fn(req)
	}

}

type ec2ErrorResponse struct {
	XMLName   xml.Name `xml:"Response"`
	Type      string   `xml:"Errors>Error>Type"`
	Code      string   `xml:"Errors>Error>Code"`
	Message   string   `xml:"Errors>Error>Message"`
	RequestID string   `xml:"RequestID"`
}

func main() {

	creds := aws.DetectCreds("these", "don't", "matter")

	mRound := NewMockRoundTripper()

	mRound.Handlers["DescribeInstances"] = returnXML(ec2.DescribeInstancesResult{
		Reservations: []ec2.Reservation{ec2.Reservation{
			Instances: []ec2.Instance{
				ec2.Instance{InstanceID: aws.String("i-inst1")},
				ec2.Instance{InstanceID: aws.String("i-inst2")},
				ec2.Instance{InstanceID: aws.String("i-inst3")},
			},
		}},
	})

	mockClient := &http.Client{Transport: mRound}

	api := ec2.New(creds, "us-west-2", mockClient)

	resp, err := api.DescribeInstances(&ec2.DescribeInstancesRequest{})

	if err != nil {
		fmt.Println("Error Response:", err)
	} else {
		fmt.Println(len(resp.Reservations[0].Instances))
	}
}

// returnXML creates a MockHandler that always returns a specific piece
// of XML
func returnXML(x interface{}) MockHandler {
	return func(req *http.Request) (*http.Response, error) {
		body, err := xml.Marshal(x)
		if err != nil {
			return nil, err
		}

		bodyReader := bytes.NewReader(body)
		resp := &http.Response{
			Status:     http.StatusText(http.StatusOK),
			StatusCode: http.StatusOK,
			Proto:      req.Proto,
			ProtoMajor: req.ProtoMajor,
			ProtoMinor: req.ProtoMinor,
			Body:       ioutil.NopCloser(bodyReader),
		}

		return resp, nil
	}
}
