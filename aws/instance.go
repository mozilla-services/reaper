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

	"github.com/milescrabill/reaper/filters"
	"github.com/milescrabill/reaper/reapable"
	log "github.com/milescrabill/reaper/reaperlog"
	"github.com/milescrabill/reaper/state"
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
			ID:     reapable.ID(*instance.InstanceID),
			Region: reapable.Region(region), // passed in cause not possible to extract out of api
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
		i.AWSState = pending
	case 16:
		i.AWSState = running
	case 32:
		i.AWSState = shuttingDown
	case 48:
		i.AWSState = terminated
	case 64:
		i.AWSState = stopping
	case 80:
		i.AWSState = stopped
	}

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
			time.Now().Add(config.Notifications.FirstStateDuration.Duration),
			state.FirstState)
	}

	return &i
}

func (i *Instance) reapableEventHTML(text string) *bytes.Buffer {
	t := htmlTemplate.Must(htmlTemplate.New("reapable").Parse(text))
	buf := bytes.NewBuffer(nil)

	data, err := i.getTemplateData()
	err = t.Execute(buf, data)
	if err != nil {
		log.Debug(fmt.Sprintf("Template generation error: %s", err))
	}
	return buf
}

func (i *Instance) reapableEventText(text string) *bytes.Buffer {
	t := textTemplate.Must(textTemplate.New("reapable").Parse(text))
	buf := bytes.NewBuffer(nil)

	data, err := i.getTemplateData()
	if err != nil {
		log.Error(fmt.Sprintf("%s", err.Error()))
	}
	err = t.Execute(buf, data)
	if err != nil {
		log.Debug(fmt.Sprintf("Template generation error: %s", err))
	}
	return buf
}

func (i *Instance) ReapableEventText() *bytes.Buffer {
	return i.reapableEventText(reapableInstanceEventText)
}

func (i *Instance) ReapableEventTextShort() *bytes.Buffer {
	return i.reapableEventText(reapableInstanceEventTextShort)
}

func (i *Instance) ReapableEventEmail() (owner mail.Address, subject string, body string, err error) {
	// if unowned, return unowned error
	if !i.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", i.ReapableDescriptionShort())}
		return
	}
	subject = fmt.Sprintf("AWS Resource %s is going to be Reaped!", i.ReapableDescriptionTiny())
	owner = *i.Owner()
	body = i.reapableEventHTML(reapableInstanceEventHTML).String()
	return
}

func (i *Instance) ReapableEventEmailShort() (owner mail.Address, body string, err error) {
	// if unowned, return unowned error
	if !i.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", i.ReapableDescriptionShort())}
		return
	}
	owner = *i.Owner()
	body = i.reapableEventHTML(reapableInstanceEventHTMLShort).String()
	return
}

type InstanceEventData struct {
	Config        *AWSConfig
	Instance      *Instance
	TerminateLink string
	StopLink      string
	WhitelistLink string
	IgnoreLink1   string
	IgnoreLink3   string
	IgnoreLink7   string
}

func (i *Instance) getTemplateData() (*InstanceEventData, error) {
	ignore1, err := MakeIgnoreLink(i.Region, i.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL, time.Duration(1*24*time.Hour))
	ignore3, err := MakeIgnoreLink(i.Region, i.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL, time.Duration(3*24*time.Hour))
	ignore7, err := MakeIgnoreLink(i.Region, i.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL, time.Duration(7*24*time.Hour))
	terminate, err := MakeTerminateLink(i.Region, i.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL)
	stop, err := MakeStopLink(i.Region, i.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL)
	whitelist, err := MakeWhitelistLink(i.Region, i.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL)

	if err != nil {
		return nil, err
	}

	return &InstanceEventData{
		Config:        config,
		Instance:      i,
		TerminateLink: terminate,
		StopLink:      stop,
		WhitelistLink: whitelist,
		IgnoreLink1:   ignore1,
		IgnoreLink3:   ignore3,
		IgnoreLink7:   ignore7,
	}, nil
}

const reapableInstanceEventHTML = `
<html>
<body>
	<p>Your AWS Instance <a href="{{ .Instance.AWSConsoleURL }}">{{ if .Instance.Name }}"{{.Instance.Name}}" {{ end }}{{.Instance.ID}} in {{.Instance.Region}}</a> is scheduled to be terminated.</p>

	<p>
		You can ignore this message and your instance will advance to the next state after <strong>{{.Instance.ReaperState.Until}}</strong>. If you do not act, it will be terminated!
	</p>

	<p>
		You may also choose to:
		<ul>
			<li><a href="{{ .TerminateLink }}">Terminate it now</a></li>
			<li><a href="{{ .StopLink }}">Stop it now</a></li>
			<li><a href="{{ .IgnoreLink1 }}">Ignore it for 1 more day</a></li>
			<li><a href="{{ .IgnoreLink3 }}">Ignore it for 3 more days</a></li>
			<li><a href="{{ .IgnoreLink7}}">Ignore it for 7 more days</a></li>
		</ul>
	</p>

	<p>
		If you want the Reaper to ignore this instance tag it with {{ .Config.WhitelistTag }} with any value, or click <a href="{{ .WhitelistLink }}">here</a>.
	</p>
</body>
</html>
`

const reapableInstanceEventHTMLShort = `
<html>
<body>
	<p>Instance <a href="{{ .Instance.AWSConsoleURL }}">{{ if .Instance.Name }}"{{.Instance.Name}}" {{ end }}{{.Instance.ID}}</a> in {{.Instance.Region}} is scheduled to be terminated after <strong>{{.Instance.ReaperState.Until}}</strong>.
		<br />
		<a href="{{ .TerminateLink }}">Terminate</a>, 
		<a href="{{ .StopLink }}">Stop</a>, 
		<a href="{{ .IgnoreLink1 }}">Ignore it for 1 more day</a>, 
		<a href="{{ .IgnoreLink3 }}">3 days</a>, 
		<a href="{{ .IgnoreLink7}}"> 7 days</a>, or 
		<a href="{{ .WhitelistLink }}">Whitelist</a> it.
	</p>
</body>
</html>
`

const reapableInstanceEventTextShort = `%%%
Instance {{if .Instance.Name}}"{{.Instance.Name}}" {{end}}[{{.Instance.ID}}]({{.Instance.AWSConsoleURL}}) in region: [{{.Instance.Region}}](https://{{.Instance.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.Instance.Region}}).{{if .Instance.Owned}} Owned by {{.Instance.Owner}}.{{end}}\n
Instance Type: {{ .Instance.InstanceType}}, {{ .Instance.AWSState.String}}{{ if .Instance.PublicIPAddress.String}}, Public IP: {{.Instance.PublicIPAddress}}.\n{{end}}
[Whitelist]({{ .WhitelistLink }}), [Stop]({{ .StopLink }}), [Terminate]({{ .TerminateLink }}) this instance.
%%%`

const reapableInstanceEventText = `%%%
Reaper has discovered an instance qualified as reapable: {{if .Instance.Name}}"{{.Instance.Name}}" {{end}}[{{.Instance.ID}}]({{.Instance.AWSConsoleURL}}) in region: [{{.Instance.Region}}](https://{{.Instance.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.Instance.Region}}).\n
{{if .Instance.Owner}}Owned by {{.Instance.Owner}}.\n{{end}}
State: {{ .Instance.AWSState.String}}.\n
Instance Type: {{ .Instance.InstanceType}}.\n
{{ if .Instance.PublicIPAddress.String}}This instance's public IP: {{.Instance.PublicIPAddress}}\n{{end}}
{{ if .Instance.AWSConsoleURL}}{{.Instance.AWSConsoleURL}}\n{{end}}
[Whitelist]({{ .WhitelistLink }}) this instance.
[Stop]({{ .StopLink }}) this instance.
[Terminate]({{ .TerminateLink }}) this instance.
%%%`

func (i *Instance) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/v2/home?region=%s#Instances:instanceId=%s",
		string(i.Region), string(i.Region), url.QueryEscape(string(i.ID))))
	if err != nil {
		log.Error(fmt.Sprintf("Error generating AWSConsoleURL. %s", err))
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
	case "TagNotEqual":
		if i.Tag(filter.Arguments[0]) != filter.Arguments[1] {
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
	case "LaunchTimeInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && time.Since(i.LaunchTime) < d {
			matched = true
		}
	case "LaunchTimeNotInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && time.Since(i.LaunchTime) > d {
			matched = true
		}
	case "ReaperState":
		if i.reaperState.State.String() == filter.Arguments[0] {
			matched = true
		}
	default:
		log.Error(fmt.Sprintf("No function %s could be found for filtering Instances.", filter.Function))
	}
	return matched
}

func (i *Instance) Terminate() (bool, error) {
	log.Notice("Terminating Instance %s", i.ReapableDescriptionTiny())
	api := ec2.New(&aws.Config{Region: string(i.Region)})
	req := &ec2.TerminateInstancesInput{
		InstanceIDs: []*string{aws.String(string(i.ID))},
	}

	resp, err := api.TerminateInstances(req)

	if err != nil {
		return false, err
	}

	if len(resp.TerminatingInstances) != 1 {
		return false, fmt.Errorf("Instance could %s not be terminated.", i.ReapableDescriptionTiny())
	}

	return true, nil
}

func (i *Instance) ForceStop() (bool, error) {
	return i.Stop()
}

func (i *Instance) Stop() (bool, error) {
	log.Notice("Stopping Instance %s", i.ReapableDescriptionTiny())
	api := ec2.New(&aws.Config{Region: string(i.Region)})
	req := &ec2.StopInstancesInput{
		InstanceIDs: []*string{aws.String(string(i.ID))},
	}

	resp, err := api.StopInstances(req)

	if err != nil {
		return false, err
	}

	if len(resp.StoppingInstances) != 1 {
		return false, fmt.Errorf("Instance %s could not be stopped.", i.ReapableDescriptionTiny())
	}

	return true, nil
}
