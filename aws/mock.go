package aws

import (
	"bytes"
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"net/url"
)

// mocking tools for testing aws libs

func init() {
}

func MockNotImplemented(req *http.Request) *http.Response {

	action := req.URL.String()
	if len(req.Form["Action"]) > 0 {
		action = req.Form["Action"][0]
	}

	x := ec2ErrorResponse{
		XMLName:   xml.Name{"Space", "Local"},
		Type:      "Not Implemented",
		Code:      "501",
		Message:   "No Mock Implementation for: " + action,
		RequestID: "000",
	}

	bodyR, _ := xml.Marshal(x)

	return &http.Response{
		Status:     http.StatusText(http.StatusNotImplemented),
		StatusCode: http.StatusNotImplemented,
		Proto:      req.Proto,
		ProtoMajor: req.ProtoMajor,
		ProtoMinor: req.ProtoMinor,
		Body:       ioutil.NopCloser(bytes.NewReader(bodyR)),
	}

}

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
		return MockNotImplemented(req), nil
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

// ReturnXML creates a MockHandler that always returns a specific piece
// of XML
func ReturnXML(x interface{}) MockHandler {
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
