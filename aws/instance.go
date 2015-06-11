package aws

import (
	"bytes"
	"fmt"
	htmlTemplate "html/template"
	"net"
	"net/mail"
	"net/url"
	textTemplate "text/template"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/mostlygeek/reaper/events"
	"github.com/mostlygeek/reaper/filters"
	"github.com/mostlygeek/reaper/reapable"
	"github.com/mostlygeek/reaper/state"
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
		i.ResourceState = pending
	case 16:
		i.ResourceState = running
	case 32:
		i.ResourceState = shuttingDown
	case 48:
		i.ResourceState = terminated
	case 64:
		i.ResourceState = stopping
	case 80:
		i.ResourceState = stopped
	}

	// TODO: untested
	if instance.PublicIPAddress != nil {
		i.PublicIPAddress = net.ParseIP(*instance.PublicIPAddress)
	}

	i.Name = i.Tag("Name")

	if i.Tagged(reaperTag) {
		// restore previously tagged state
		i.reaperState = state.NewStateWithTag(i.Tags[reaperTag])
	} else {
		// initial state
		i.reaperState = state.NewStateWithUntilAndState(
			time.Now().Add(Config.Notifications.FirstNotification.Duration),
			state.STATE_START)
	}

	return &i
}

func (i *Instance) ReapableEventText() *bytes.Buffer {
	t := textTemplate.Must(textTemplate.New("reapable-instance").Funcs(ReapableEventFuncMap).Parse(reapableInstanceEventText))
	buf := bytes.NewBuffer(nil)

	// anonymous struct
	data := struct {
		Config   *events.HTTPConfig
		Instance *Instance
	}{
		Instance: i,
		Config:   &Config.HTTP,
	}
	err := t.Execute(buf, data)
	if err != nil {
		Log.Debug("Template generation error", err)
	}
	return buf
}

func (i *Instance) ReapableEventEmail() (owner mail.Address, subject string, body string, err error) {
	// if unowned, return unowned error
	if !i.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s in region %s does not have an owner tag", i.ID, i.Region)}
		return
	}

	t := htmlTemplate.Must(htmlTemplate.New("reapable-instance").Funcs(htmlTemplate.FuncMap(ReapableEventFuncMap)).Parse(reapableInstanceEventHTML))
	buf := bytes.NewBuffer(nil)

	// anonymous struct
	data := struct {
		Config   *events.HTTPConfig
		Instance *Instance
		Delay1   time.Duration
		Delay3   time.Duration
		Delay7   time.Duration
	}{
		Instance: i,
		Config:   &Config.HTTP,
		Delay1:   time.Duration(24 * time.Hour),
		Delay3:   time.Duration(3 * 24 * time.Hour),
		Delay7:   time.Duration(7 * 24 * time.Hour),
	}
	err = t.Execute(buf, data)
	if err != nil {
		return
	}
	subject = fmt.Sprintf("An AWS Resource (%s in region %s) you own is going to be Reaped!", i.ID, i.Region)
	owner = *i.Owner()
	body = buf.String()
	return
}

// TODO: pass values instead of functions -_-
const reapableInstanceEventHTML = `
<html>
<body>
	<p>Your AWS Resource {{ if .Instance.Name }}"{{.Instance.Name}}" {{ end }}{{.Instance.ID}} in {{.Instance.Region}} is scheduled to be terminated.</p>

	<p>
		You can ignore this message and your instance will be automatically
		terminated after <strong>{{.Instance.ReaperState.Until}}</strong>.
	</p>

	<p>
		You may also choose to:
		<ul>
			<li><a href="{{ MakeTerminateLink .Config.HTTP.TokenSecret .Config.HTTP.HTTPApiURL .Instance.Region .Instance.ID }}">Terminate it now</a></li>
			<li><a href="{{ MakeStopLink .Config.HTTP.TokenSecret .Config.HTTP.HTTPApiURL .Instance.Region .Instance.ID }}">Stop it</a></li>
			<li><a href="{{ MakeIgnoreLink .Config.HTTP.TokenSecret .Config.HTTP.HTTPApiURL .Instance.Region .Instance.ID .Delay1 }}">Ignore it for 1 more day</a></li>
			<li><a href="{{ MakeIgnoreLink .Config.HTTP.TokenSecret .Config.HTTP.HTTPApiURL .Instance.Region .Instance.ID .Delay3 }}">Ignore it for 3 more days</a></li>
			<li><a href="{{ MakeIgnoreLink .Config.HTTP.TokenSecret .Config.HTTP.HTTPApiURL .Instance.Region .Instance.ID .Delay7}}">Ignore it for 7 more days</a></li>
		</ul>
	</p>

	<p>
		If you want the Reaper to ignore this instance tag it with {{ .Config.WhitelistTag }} with any value, or click <a href="{{ MakeWhitelistLink .Config.HTTP.TokenSecret .Config.HTTP.HTTPApiURL .Instance.Region .Instance.ID }}">here</a>.
	</p>
</body>
</html>
`

// TODO: pass values instead of functions -_-
const reapableInstanceEventText = `%%%
Reaper has discovered an instance qualified as reapable: {{if .Instance.Name}}"{{.Instance.Name}}" {{end}}[{{.Instance.ID}}]({{.Instance.AWSConsoleURL}}) in region: [{{.Instance.Region}}](https://{{.Instance.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.Instance.Region}}).\n
{{if .Instance.Owned}}Owned by {{.Instance.Owner}}.\n{{end}}
State: {{ .Instance.ResourceState.String}}.\n
Instance Type: {{ .Instance.InstanceType}}.\n
{{ if .Instance.PublicIPAddress.String}}This instance's public IP: {{.Instance.PublicIPAddress}}\n{{end}}
{{ if .Instance.AWSConsoleURL}}{{.Instance.AWSConsoleURL}}\n{{end}}
[AWS Console URL]({{.Instance.AWSConsoleURL}})\n
[Whitelist]({{ MakeWhitelistLink .Config.HTTP.TokenSecret .Config.HTTP.HTTPApiURL .Instance.Region .Instance.ID }}) this instance.
[Stop]({{ MakeStopLink .Config.HTTP.TokenSecret .Config.HTTP.HTTPApiURL .Instance.Region .Instance.ID }}) this instance.
[Terminate]({{ MakeTerminateLink .Config.HTTP.TokenSecret .Config.HTTP.HTTPApiURL .Instance.Region .Instance.ID }}) this instance.
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

func (i *Instance) Filter(filter filters.Filter) bool {
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
func (i *Instance) Save(s *state.State) (bool, error) {
	// if !i.Tagged(reaperTag) {
	// 	Log.Info("Set Reaper start state on %s in region %s. New tag: %s.", i.ID, i.Region, i.reaperState.String())
	// 	return i.TagReaperState(i.reaperState)
	// }
	return i.TagReaperState(s)
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
