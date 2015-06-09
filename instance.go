package main

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const (
	reaperTag           = "REAPER"
	reaperTagSeparator  = "|"
	reaperTagTimeFormat = "2006-01-02 03:04PM MST"
)

// Instance stores data from an *ec2.Instance
type Instance struct {
	AWSResource
	LaunchTime      time.Time
	SecurityGroups  map[string]string
	InstanceType    string
	PublicIPAddress net.IP
}

// NewInstance is a constructor for Instances
func NewInstance(region string, instance *ec2.Instance) *Instance {
	i := Instance{
		AWSResource: AWSResource{
			ID:     *instance.InstanceID,
			Region: region, // passed in cause not possible to extract out of api
			Tags:   make(map[string]string),
		},

		SecurityGroups: make(map[string]string),
		LaunchTime:     *instance.LaunchTime,
		InstanceType:   *instance.InstanceType,
	}

	for _, sg := range instance.SecurityGroups {
		i.SecurityGroups[*sg.GroupID] = *sg.GroupName
	}

	for _, tag := range instance.Tags {
		i.Tags[*tag.Key] = *tag.Value
	}

	switch *instance.State.Code {
	case 0:
		i.resourceState = pending
	case 16:
		i.resourceState = running
	case 32:
		i.resourceState = shuttingDown
	case 48:
		i.resourceState = terminated
	case 64:
		i.resourceState = stopping
	case 80:
		i.resourceState = stopped
	}

	// TODO: untested
	if instance.PublicIPAddress != nil {
		i.PublicIPAddress = net.ParseIP(*instance.PublicIPAddress)
	}

	i.Name = i.Tag("Name")
	i.reaperState = ParseState(i.Tags[reaperTag])

	return &i
}

type InstanceEventData struct {
	Config   *Config
	Instance *Instance
}

func (i *Instance) ReapableEventText() *bytes.Buffer {
	t := template.Must(template.New("reapable-instance").Funcs(ReapableEventFuncMap).Parse(reapableInstanceEventText))
	buf := bytes.NewBuffer(nil)

	data := InstanceEventData{
		Instance: i,
		Config:   Conf,
	}
	err := t.Execute(buf, data)
	if err != nil {
		Log.Debug("Template generation error", err)
	}
	return buf
}

const reapableInstanceEventText = `%%%
Reaper has discovered an instance qualified as reapable: {{if .Instance.Name}}"{{.Instance.Name}}" {{end}}[{{.Instance.ID}}]({{.Instance.AWSConsoleURL}}) in region: [{{.Instance.Region}}](https://{{.Instance.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.Instance.Region}}).\n
{{if .Instance.Owned}}Owned by {{.Instance.Owner}}.\n{{end}}
State: {{ .Instance.State.String}}.\n
Instance Type: {{ .Instance.InstanceType}}.\n
{{ if .Instance.PublicIPAddress.String}}This instance's public IP: {{.Instance.PublicIPAddress}}\n{{end}}
{{ if .Instance.AWSConsoleURL}}{{.Instance.AWSConsoleURL}}\n{{end}}
[AWS Console URL]({{.Instance.AWSConsoleURL}})\n
[Whitelist]({{ MakeWhitelistLink .Config.TokenSecret .Config.HTTPApiURL .Instance.Region .Instance.ID }}) this instance.
[Stop]({{ MakeStopLink .Config.TokenSecret .Config.HTTPApiURL .Instance.Region .Instance.ID }}) this instance.
[Terminate]({{ MakeTerminateLink .Config.TokenSecret .Config.HTTPApiURL .Instance.Region .Instance.ID }}) this instance.
%%%`

func (i *Instance) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/v2/home?region=%s#Instances:instanceId=%s",
		i.Region, i.Region, i.ID))
	if err != nil {
		Log.Error(fmt.Sprintf("Error generating AWSConsoleURL. %s", err))
	}
	return url
}

// Autoscaled checks if the instance is part of an autoscaling group
func (i *Instance) AutoScaled() (ok bool) { return i.Tagged("aws:autoscaling:groupName") }

func (i *Instance) Filter(filter Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
	case "Pending":
		if b, err := filter.BoolValue(0); err == nil && i.Pending() == b {
			matched = true
		}
	case "Running":
		if b, err := filter.BoolValue(0); err == nil && i.Running() == b {
			matched = true
		}
	case "ShuttingDown":
		if b, err := filter.BoolValue(0); err == nil && i.ShuttingDown() == b {
			matched = true
		}
	case "Terminated":
		if b, err := filter.BoolValue(0); err == nil && i.Terminated() == b {
			matched = true
		}
	case "Stopping":
		if b, err := filter.BoolValue(0); err == nil && i.Stopping() == b {
			matched = true
		}
	case "Stopped":
		if b, err := filter.BoolValue(0); err == nil && i.Stopped() == b {
			matched = true
		}
	case "InstanceType":
		if i.InstanceType == filter.Arguments[0] {
			matched = true
		}
	case "Tagged":
		if i.Tagged(filter.Arguments[0]) {
			matched = true
		}
	case "NotTagged":
		if !i.Tagged(filter.Arguments[0]) {
			matched = true
		}
	case "Tag":
		if i.Tag(filter.Arguments[0]) == filter.Arguments[1] {
			matched = true
		}
	case "HasPublicIPAddress":
		if i.PublicIPAddress != nil {
			matched = true
		}
	case "PublicIPAddress":
		if i.PublicIPAddress.String() == filter.Arguments[0] {
			matched = true
		}
	// uses RFC3339 format
	// https://www.ietf.org/rfc/rfc3339.txt
	case "LaunchTimeBefore":
		t, err := time.Parse(time.RFC3339, filter.Arguments[0])
		if err == nil && t.After(i.LaunchTime) {
			matched = true
		}
	case "LaunchTimeAfter":
		t, err := time.Parse(time.RFC3339, filter.Arguments[0])
		if err == nil && t.Before(i.LaunchTime) {
			matched = true
		}
	case "ReaperState":
		// one of:
		// notify1
		// notify2
		// ignore
		// start
		if i.reaperState.State.String() == filter.Arguments[0] {
			matched = true
		}
	default:
		Log.Error("No function %s could be found for filtering ASGs.", filter.Function)
	}
	return matched
}

// methods for reapable interface:
func (i *Instance) Save(state *State) (bool, error) {
	return i.TagReaperState(state)
}

func (i *Instance) Terminate() (bool, error) {
	api := ec2.New(&aws.Config{Region: i.Region})
	req := &ec2.TerminateInstancesInput{
		InstanceIDs: []*string{aws.String(i.ID)},
	}

	resp, err := api.TerminateInstances(req)

	if err != nil {
		return false, err
	}

	if len(resp.TerminatingInstances) != 1 {
		return false, fmt.Errorf("Instance could %s not be terminated.", i.ID)
	}

	return true, nil
}

func (i *Instance) ForceStop() (bool, error) {
	return i.Stop()
}

func (i *Instance) Stop() (bool, error) {
	api := ec2.New(&aws.Config{Region: i.Region})
	req := &ec2.StopInstancesInput{
		InstanceIDs: []*string{aws.String(i.ID)},
	}

	resp, err := api.StopInstances(req)

	if err != nil {
		return false, err
	}

	if len(resp.StoppingInstances) != 1 {
		return false, fmt.Errorf("Instance %s could not be stopped.", i.ID)
	}

	return true, nil
}
