package aws

import (
	"bytes"
	"fmt"
	htmlTemplate "html/template"
	"net/mail"
	"net/url"
	"strings"
	textTemplate "text/template"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

// Instance is a is a Reapable, Filterable
// embeds AWS API's ec2.Instance
type Instance struct {
	Resource
	ec2.Instance
	SecurityGroups map[reapable.ID]string
	AutoScaled     bool
	Scheduling     instanceScalingSchedule
}

type instanceScalingSchedule struct {
	Enabled         bool
	ScaleDownString string
	ScaleUpString   string
}

// SaveSchedule is a method of the Scaler interface
func (a *Instance) SaveSchedule() {
	tag(a.Region.String(), a.ID.String(), scalerTag, a.Scheduling.scheduleTag())
}

// SetScaleDownString is a method of the Scaler interface
func (a *Instance) SetScaleDownString(s string) {
	a.Scheduling.ScaleDownString = s
}

// SetScaleUpString is a method of the Scaler interface
func (a *Instance) SetScaleUpString(s string) {
	a.Scheduling.ScaleUpString = s
}

func (s *instanceScalingSchedule) setSchedule(tag string) {
	// scalerTag format: cron format schedule (scale down),cron format schedule (scale up),previous scale time,previous desired size,previous min size
	splitTag := strings.Split(tag, ",")
	if len(splitTag) != 2 {
		log.Error(fmt.Sprintf("Invalid Instance Tag format %s", tag))
	} else {
		s.ScaleDownString = splitTag[0]
		s.ScaleUpString = splitTag[1]
		s.Enabled = true
	}
}

func (s instanceScalingSchedule) scheduleTag() string {
	return strings.Join([]string{
		// keep the same schedules
		s.ScaleDownString,
		s.ScaleUpString,
	}, ",")
}

// NewInstance creates an Instance from the AWS API's ec2.Instance
func NewInstance(region string, instance *ec2.Instance) *Instance {
	a := Instance{
		Resource: Resource{
			ID:     reapable.ID(*instance.InstanceID),
			Region: reapable.Region(region), // passed in cause not possible to extract out of api
			Tags:   make(map[string]string),
		},
		SecurityGroups: make(map[reapable.ID]string),
		Instance:       *instance,
	}

	for _, sg := range instance.SecurityGroups {
		if sg != nil {
			a.SecurityGroups[reapable.ID(*sg.GroupID)] = *sg.GroupName
		}
	}

	for _, tag := range instance.Tags {
		a.Resource.Tags[*tag.Key] = *tag.Value
	}

	// Scaler boilerplate
	if a.Tagged(scalerTag) {
		a.Scheduling.setSchedule(a.Tag(scalerTag))
	}

	if a.Tagged("aws:autoscaling:groupName") {
		a.AutoScaled = true
	}

	a.Name = a.Tag("Name")

	if a.Tagged(reaperTag) {
		// restore previously tagged state
		a.reaperState = state.NewStateWithTag(a.Tag(reaperTag))
	} else {
		// initial state
		a.reaperState = state.NewStateWithUntilAndState(
			time.Now().Add(config.Notifications.FirstStateDuration.Duration),
			state.FirstState)
	}

	return &a
}

// Pending returns whether an instance's State is Pending
func (a *Instance) Pending() bool { return *a.State.Code == 0 }

// Running returns whether an instance's State is Running
func (a *Instance) Running() bool { return *a.State.Code == 16 }

// ShuttingDown returns whether an instance's State is ShuttingDown
func (a *Instance) ShuttingDown() bool { return *a.State.Code == 32 }

// Terminated returns whether an instance's State is Terminated
func (a *Instance) Terminated() bool { return *a.State.Code == 48 }

// Stopping returns whether an instance's State is Stopping
func (a *Instance) Stopping() bool { return *a.State.Code == 64 }

// Stopped returns whether an instance's State is Stopped
func (a *Instance) Stopped() bool { return *a.State.Code == 80 }

func (a *Instance) reapableEventHTML(text string) *bytes.Buffer {
	t := htmlTemplate.Must(htmlTemplate.New("reapable").Parse(text))
	buf := bytes.NewBuffer(nil)

	data, err := a.getTemplateData()
	err = t.Execute(buf, data)
	if err != nil {
		log.Debug(fmt.Sprintf("Template generation error: %s", err))
	}
	return buf
}

func (a *Instance) reapableEventText(text string) *bytes.Buffer {
	t := textTemplate.Must(textTemplate.New("reapable").Parse(text))
	buf := bytes.NewBuffer(nil)

	data, err := a.getTemplateData()
	if err != nil {
		log.Error(fmt.Sprintf("%s", err.Error()))
	}
	err = t.Execute(buf, data)
	if err != nil {
		log.Debug(fmt.Sprintf("Template generation error: %s", err))
	}
	return buf
}

// ReapableEventText is part of the events.Reapable interface
func (a *Instance) ReapableEventText() *bytes.Buffer {
	return a.reapableEventText(reapableInstanceEventText)
}

// ReapableEventTextShort is part of the events.Reapable interface
func (a *Instance) ReapableEventTextShort() *bytes.Buffer {
	return a.reapableEventText(reapableInstanceEventTextShort)
}

// ReapableEventEmail is part of the events.Reapable interface
func (a *Instance) ReapableEventEmail() (owner mail.Address, subject string, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}
	subject = fmt.Sprintf("AWS Resource %s is going to be Reaped!", a.ReapableDescriptionTiny())
	owner = *a.Owner()
	body = a.reapableEventHTML(reapableInstanceEventHTML)
	return
}

// ReapableEventEmailShort is part of the events.Reapable interface
func (a *Instance) ReapableEventEmailShort() (owner mail.Address, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}
	owner = *a.Owner()
	body = a.reapableEventHTML(reapableInstanceEventHTMLShort)
	return
}

type instanceEventData struct {
	Config                           *Config
	Instance                         *Instance
	TerminateLink                    string
	StopLink                         string
	WhitelistLink                    string
	IgnoreLink1                      string
	IgnoreLink3                      string
	IgnoreLink7                      string
	SchedulePacificBusinessHoursLink string
	ScheduleEasternBusinessHoursLink string
	ScheduleCESTBusinessHoursLink    string
}

func (a *Instance) getTemplateData() (*instanceEventData, error) {
	ignore1, err := makeIgnoreLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.APIURL, time.Duration(1*24*time.Hour))
	if err != nil {
		return nil, err
	}
	ignore3, err := makeIgnoreLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.APIURL, time.Duration(3*24*time.Hour))
	if err != nil {
		return nil, err
	}
	ignore7, err := makeIgnoreLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.APIURL, time.Duration(7*24*time.Hour))
	if err != nil {
		return nil, err
	}
	terminate, err := makeTerminateLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.APIURL)
	if err != nil {
		return nil, err
	}
	stop, err := makeStopLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.APIURL)
	if err != nil {
		return nil, err
	}
	whitelist, err := makeWhitelistLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.APIURL)
	if err != nil {
		return nil, err
	}
	schedulePacific, err := makeScheduleLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.APIURL, scaleDownPacificBusinessHours, scaleUpPacificBusinessHours)
	if err != nil {
		return nil, err
	}
	scheduleEastern, err := makeScheduleLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.APIURL, scaleDownEasternBusinessHours, scaleUpEasternBusinessHours)
	if err != nil {
		return nil, err
	}
	scheduleCEST, err := makeScheduleLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.APIURL, scaleDownCESTBusinessHours, scaleUpCESTBusinessHours)
	if err != nil {
		return nil, err
	}

	// return the data
	return &instanceEventData{
		Config:                           config,
		Instance:                         a,
		TerminateLink:                    terminate,
		StopLink:                         stop,
		WhitelistLink:                    whitelist,
		IgnoreLink1:                      ignore1,
		IgnoreLink3:                      ignore3,
		IgnoreLink7:                      ignore7,
		SchedulePacificBusinessHoursLink: schedulePacific,
		ScheduleEasternBusinessHoursLink: scheduleEastern,
		ScheduleCESTBusinessHoursLink:    scheduleCEST,
	}, nil
}

// ScaleDown stops an Instance
func (a *Instance) ScaleDown() {
	if !a.Running() {
		return
	}
	_, err := a.Stop()
	if err != nil {
		log.Error(err.Error())
	}
}

// ScaleUp starts an Instance
func (a *Instance) ScaleUp() {
	if !a.Stopped() {
		return
	}
	_, err := a.Start()
	if err != nil {
		log.Error(err.Error())
	}
}

const reapableInstanceEventHTML = `
<html>
<body>
	<p>Your AWS Instance <a href="{{ .Instance.AWSConsoleURL }}">{{ if .Instance.Name }}"{{.Instance.Name}}" {{ end }}{{.Instance.ID}} in {{.Instance.Region}}</a> is scheduled to be terminated.</p>

	<p>
		You can ignore this message and your instance will advance to the next state after <strong>{{.Instance.ReaperState.Until.UTC.Format "Jan 2, 2006 at 3:04pm (MST)"}}</strong>. If you do not take action it will be terminated!
	</p>

	<p>
		You may also choose to:
		<ul>
			<li><a href="{{ .TerminateLink }}">Terminate it now</a></li>
			<li><a href="{{ .StopLink }}">Stop it now</a></li>
			<li><a href="{{ .IgnoreLink1 }}">Ignore it for 1 more day</a></li>
			<li><a href="{{ .IgnoreLink3 }}">Ignore it for 3 more days</a></li>
			<li><a href="{{ .IgnoreLink7}}">Ignore it for 7 more days</a></li>
			<li><a href="{{ .SchedulePacificBusinessHoursLink}}">Schedule it to start and stop with Pacific business hours</a></li>
			<li><a href="{{ .ScheduleEasternBusinessHoursLink}}">Schedule it to start and stop with Eastern business hours</a></li>
			<li><a href="{{ .ScheduleCESTBusinessHoursLink}}">Schedule it to start and stop with CEST business hours</a></li>
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
	<p>Instance <a href="{{ .Instance.AWSConsoleURL }}">{{ if .Instance.Name }}"{{.Instance.Name}}" {{ end }}{{.Instance.ID}}</a> in {{.Instance.Region}} is scheduled to be terminated after <strong>{{.Instance.ReaperState.Until.UTC.Format "Jan 2, 2006 at 3:04pm (MST)"}}</strong>.
		<br />
		Schedule it to start and stop with <a href="{{ .SchedulePacificBusinessHoursLink}}">Pacific</a>, 
		<a href="{{ .ScheduleEasternBusinessHoursLink}}">Eastern</a>, or 
		<a href="{{ .ScheduleCESTBusinessHoursLink}}">CEST</a> business hours, 
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
Instance Type: {{ .Instance.InstanceType}}, {{ .Instance.State.Name}}{{ if .Instance.PublicIPAddress}}, Public IP: {{.Instance.PublicIPAddress}}.\n{{end}}
Schedule this instance to stop and start with [Pacific]({{ .SchedulePacificBusinessHoursLink}}), [Eastern]({{ .ScheduleEasternBusinessHoursLink}}), or [CEST]({{ .ScheduleCESTBusinessHoursLink}}) business hours.\n
[Whitelist]({{ .WhitelistLink }}), [Stop]({{ .StopLink }}), or [Terminate]({{ .TerminateLink }}) this instance.
%%%`

const reapableInstanceEventText = `%%%
Reaper has discovered an instance qualified as reapable: {{if .Instance.Name}}"{{.Instance.Name}}" {{end}}[{{.Instance.ID}}]({{.Instance.AWSConsoleURL}}) in region: [{{.Instance.Region}}](https://{{.Instance.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.Instance.Region}}).\n
{{if .Instance.Owner}}Owned by {{.Instance.Owner}}.\n{{end}}
State: {{ .Instance.State.Name}}.\n
Instance Type: {{ .Instance.InstanceType}}.\n
{{ if .Instance.PublicIPAddress}}This instance's public IP: {{.Instance.PublicIPAddress}}\n{{end}}
{{ if .Instance.AWSConsoleURL}}{{.Instance.AWSConsoleURL}}\n{{end}}
Schedule this instance to start and stop with [Pacific]({{ .SchedulePacificBusinessHoursLink}}), [Eastern]({{ .ScheduleEasternBusinessHoursLink}}), or [CEST]({{ .ScheduleCESTBusinessHoursLink}}) business hours.\n
[Whitelist]({{ .WhitelistLink }}).
[Stop]({{ .StopLink }}) this instance.
[Terminate]({{ .TerminateLink }}) this instance.
%%%`

// AWSConsoleURL returns the url that can be used to access the resource on the AWS Console
func (a *Instance) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/v2/home?region=%s#Instances:instanceId=%s",
		string(a.Region), string(a.Region), url.QueryEscape(string(a.ID))))
	if err != nil {
		log.Error(fmt.Sprintf("Error generating AWSConsoleURL. %s", err))
	}
	return url
}

// Filter is part of the filter.Filterable interface
func (a *Instance) Filter(filter filters.Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
	case "Pending":
		// Pending returns whether an instance's State is Pending
		if b, err := filter.BoolValue(0); err == nil && a.Pending() == b {
			// Pending returns whether an instance's State is Pending
			matched = true
			// Pending returns whether an instance's State is Pending
		}
		// Pending returns whether an instance's State is Pending
	case "Running":
		// Pending returns whether an instance's State is Pending
		if b, err := filter.BoolValue(0); err == nil && a.Running() == b {
			// Pending returns whether an instance's State is Pending
			matched = true
		}
	case "ShuttingDown":
		if b, err := filter.BoolValue(0); err == nil && a.ShuttingDown() == b {
			matched = true
		}
	case "Terminated":
		if b, err := filter.BoolValue(0); err == nil && a.Terminated() == b {
			matched = true
		}
	case "Stopping":
		if b, err := filter.BoolValue(0); err == nil && a.Stopping() == b {
			matched = true
		}
	case "Stopped":
		if b, err := filter.BoolValue(0); err == nil && a.Stopped() == b {
			matched = true
		}
	case "InstanceType":
		if a.InstanceType != nil && *a.InstanceType == filter.Arguments[0] {
			matched = true
		}
	case "Tagged":
		if a.Tagged(filter.Arguments[0]) {
			matched = true
		}
	case "NotTagged":
		if !a.Tagged(filter.Arguments[0]) {
			matched = true
		}
	case "Tag":
		if a.Tag(filter.Arguments[0]) == filter.Arguments[1] {
			matched = true
		}
	case "TagNotEqual":
		if a.Tag(filter.Arguments[0]) != filter.Arguments[1] {
			matched = true
		}
	case "HasPublicIPAddress":
		if a.PublicIPAddress != nil {
			matched = true
		}
	case "PublicIPAddress":
		if a.PublicIPAddress != nil && *a.PublicIPAddress == filter.Arguments[0] {
			matched = true
		}
	// uses RFC3339 format
	// https://www.ietf.org/rfc/rfc3339.txt
	case "LaunchTimeBefore":
		t, err := time.Parse(time.RFC3339, filter.Arguments[0])
		if err == nil && a.LaunchTime != nil && t.After(*a.LaunchTime) {
			matched = true
		}
	case "LaunchTimeAfter":
		t, err := time.Parse(time.RFC3339, filter.Arguments[0])
		if err == nil && a.LaunchTime != nil && t.Before(*a.LaunchTime) {
			matched = true
		}
	case "LaunchTimeInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && a.LaunchTime != nil && time.Since(*a.LaunchTime) < d {
			matched = true
		}
	case "LaunchTimeNotInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && a.LaunchTime != nil && time.Since(*a.LaunchTime) > d {
			matched = true
		}
	case "Region":
		for region := range filter.Arguments {
			if a.Region == reapable.Region(region) {
				matched = true
			}
		}
	case "NotRegion":
		for region := range filter.Arguments {
			if a.Region == reapable.Region(region) {
				matched = false
			}
		}
	case "ReaperState":
		if a.reaperState.State.String() == filter.Arguments[0] {
			matched = true
		}
	case "InCloudformation":
		if b, err := filter.BoolValue(0); err == nil && a.IsInCloudformation == b {
			matched = true
		}
	case "AutoScaled":
		if b, err := filter.BoolValue(0); err == nil && a.AutoScaled == b {
			matched = true
		}
	case "IsDependency":
		if b, err := filter.BoolValue(0); err == nil && a.Dependency == b {
			matched = true
		}
	case "NameContains":
		if strings.Contains(a.Name, filter.Arguments[0]) {
			matched = true
		}
	default:
		log.Error(fmt.Sprintf("No function %s could be found for filtering Instances.", filter.Function))
	}
	return matched
}

// Terminate is a method of reapable.Terminable, which is embedded in reapable.Reapable
func (a *Instance) Terminate() (bool, error) {
	log.Notice("Terminating Instance %s", a.ReapableDescriptionTiny())
	api := ec2.New(&aws.Config{Region: string(a.Region)})
	req := &ec2.TerminateInstancesInput{
		InstanceIDs: []*string{aws.String(string(a.ID))},
	}

	resp, err := api.TerminateInstances(req)

	if err != nil {
		return false, err
	}

	if len(resp.TerminatingInstances) != 1 {
		return false, fmt.Errorf("Instance could %s not be terminated.", a.ReapableDescriptionTiny())
	}

	return true, nil
}

// ForceStop is a method of reapable.Stoppable, which is embedded in reapable.Reapable
func (a *Instance) ForceStop() (bool, error) {
	return a.Stop()
}

// Start starts an instance
func (a *Instance) Start() (bool, error) {
	log.Notice("Starting Instance %s", a.ReapableDescriptionTiny())
	api := ec2.New(&aws.Config{Region: string(a.Region)})
	req := &ec2.StartInstancesInput{
		InstanceIDs: []*string{aws.String(string(a.ID))},
	}

	resp, err := api.StartInstances(req)

	if err != nil {
		return false, err
	}

	if len(resp.StartingInstances) != 1 {
		return false, fmt.Errorf("Instance %s could not be started.", a.ReapableDescriptionTiny())
	}

	return true, nil
}

// Stop is a method of reapable.Stoppable, which is embedded in reapable.Reapable
func (a *Instance) Stop() (bool, error) {
	log.Notice("Stopping Instance %s", a.ReapableDescriptionTiny())
	api := ec2.New(&aws.Config{Region: string(a.Region)})
	req := &ec2.StopInstancesInput{
		InstanceIDs: []*string{aws.String(string(a.ID))},
	}

	resp, err := api.StopInstances(req)

	if err != nil {
		return false, err
	}

	if len(resp.StoppingInstances) != 1 {
		return false, fmt.Errorf("Instance %s could not be stopped.", a.ReapableDescriptionTiny())
	}

	return true, nil
}
