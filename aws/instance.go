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

// Instance stores data from an *ec2.Instance
type Instance struct {
	AWSResource
	ec2.Instance
	SecurityGroups map[reapable.ID]string
	AutoScaled     bool
	Scheduling     InstanceScalingSchedule
}

type InstanceScalingSchedule struct {
	enabled         bool
	scaleDownString string
	scaleUpString   string
}

func (s *InstanceScalingSchedule) setSchedule(tag string) {
	// scalerTag format: cron format schedule (scale down),cron format schedule (scale up),previous scale time,previous desired size,previous min size
	splitTag := strings.Split(tag, ",")
	if len(splitTag) != 2 {
		log.Error("Invalid Instance Tag format %s", tag)
	} else {
		s.scaleDownString = splitTag[0]
		s.scaleUpString = splitTag[1]
		s.enabled = true
	}
}

func (s InstanceScalingSchedule) scheduleTag() string {
	return strings.Join([]string{
		// keep the same schedules
		s.scaleDownString,
		s.scaleUpString,
	}, ",")
}

// NewInstance is a constructor for Instances
func NewInstance(region string, instance *ec2.Instance) *Instance {
	i := Instance{
		AWSResource: AWSResource{
			ID:     reapable.ID(*instance.InstanceID),
			Region: reapable.Region(region), // passed in cause not possible to extract out of api
			Tags:   make(map[string]string),
		},
		SecurityGroups: make(map[reapable.ID]string),
		Instance:       *instance,
	}

	for _, sg := range instance.SecurityGroups {
		if sg != nil {
			i.SecurityGroups[reapable.ID(*sg.GroupID)] = *sg.GroupName
		}
	}

	for _, tag := range instance.Tags {
		i.AWSResource.Tags[*tag.Key] = *tag.Value
	}

	// Scaler boilerplate
	if i.Tagged(scalerTag) {
		i.Scheduling.setSchedule(i.Tag(scalerTag))
	}

	if i.Tagged("aws:autoscaling:groupName") {
		i.AutoScaled = true
	}

	i.Name = i.Tag("Name")

	if i.Tagged(reaperTag) {
		// restore previously tagged state
		i.reaperState = state.NewStateWithTag(i.Tag(reaperTag))
	} else {
		// initial state
		i.reaperState = state.NewState()
	}

	return &i
}

func (a *Instance) Pending() bool      { return *a.State.Code == 0 }
func (a *Instance) Running() bool      { return *a.State.Code == 16 }
func (a *Instance) ShuttingDown() bool { return *a.State.Code == 32 }
func (a *Instance) Terminated() bool   { return *a.State.Code == 48 }
func (a *Instance) Stopping() bool     { return *a.State.Code == 64 }
func (a *Instance) Stopped() bool      { return *a.State.Code == 80 }

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

func (i *Instance) ReapableEventEmail() (owner mail.Address, subject string, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !i.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", i.ReapableDescriptionShort())}
		return
	}
	subject = fmt.Sprintf("AWS Resource %s is going to be Reaped!", i.ReapableDescriptionTiny())
	owner = *i.Owner()
	body = i.reapableEventHTML(reapableInstanceEventHTML)
	return
}

func (i *Instance) ReapableEventEmailShort() (owner mail.Address, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !i.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", i.ReapableDescriptionShort())}
		return
	}
	owner = *i.Owner()
	body = i.reapableEventHTML(reapableInstanceEventHTMLShort)
	return
}

type InstanceEventData struct {
	Config                           *AWSConfig
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

func (i *Instance) getTemplateData() (*InstanceEventData, error) {
	ignore1, err := MakeIgnoreLink(i.Region, i.ID, config.HTTP.TokenSecret, config.HTTP.APIURL, time.Duration(1*24*time.Hour))
	if err != nil {
		return nil, err
	}
	ignore3, err := MakeIgnoreLink(i.Region, i.ID, config.HTTP.TokenSecret, config.HTTP.APIURL, time.Duration(3*24*time.Hour))
	if err != nil {
		return nil, err
	}
	ignore7, err := MakeIgnoreLink(i.Region, i.ID, config.HTTP.TokenSecret, config.HTTP.APIURL, time.Duration(7*24*time.Hour))
	if err != nil {
		return nil, err
	}
	terminate, err := MakeTerminateLink(i.Region, i.ID, config.HTTP.TokenSecret, config.HTTP.APIURL)
	if err != nil {
		return nil, err
	}
	stop, err := MakeStopLink(i.Region, i.ID, config.HTTP.TokenSecret, config.HTTP.APIURL)
	if err != nil {
		return nil, err
	}
	whitelist, err := MakeWhitelistLink(i.Region, i.ID, config.HTTP.TokenSecret, config.HTTP.APIURL)
	if err != nil {
		return nil, err
	}
	schedulePacific, err := MakeScheduleLink(i.Region, i.ID, config.HTTP.TokenSecret, config.HTTP.APIURL, scaleDownPacificBusinessHours, scaleUpPacificBusinessHours)
	if err != nil {
		return nil, err
	}
	scheduleEastern, err := MakeScheduleLink(i.Region, i.ID, config.HTTP.TokenSecret, config.HTTP.APIURL, scaleDownEasternBusinessHours, scaleUpEasternBusinessHours)
	if err != nil {
		return nil, err
	}
	scheduleCEST, err := MakeScheduleLink(i.Region, i.ID, config.HTTP.TokenSecret, config.HTTP.APIURL, scaleDownCESTBusinessHours, scaleUpCESTBusinessHours)
	if err != nil {
		return nil, err
	}

	// return the data
	return &InstanceEventData{
		Config:                           config,
		Instance:                         i,
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

// Scaler interface
func (i *Instance) SetScaleDownString(s string) {
	i.Scheduling.scaleDownString = s
}

func (i *Instance) SetScaleUpString(s string) {
	i.Scheduling.scaleUpString = s
}

func (i *Instance) SaveSchedule() {
	tag(i.Region.String(), i.ID.String(), scalerTag, i.Scheduling.scheduleTag())
}

func (i *Instance) SchedulingEnabled() bool {
	return i.Scheduling.enabled
}

func (i *Instance) ScaleDownSchedule() string {
	return i.Scheduling.scaleDownString
}

func (i *Instance) ScaleUpSchedule() string {
	return i.Scheduling.scaleUpString
}

func (i *Instance) ScaleDown() {
	if !i.Running() {
		return
	}
	_, err := i.Stop()
	if err != nil {
		log.Error(err.Error())
	}
}

func (i *Instance) ScaleUp() {
	if !i.Stopped() {
		return
	}
	_, err := i.Start()
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

func (i *Instance) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/v2/home?region=%s#Instances:instanceId=%s",
		string(i.Region), string(i.Region), url.QueryEscape(string(i.ID))))
	if err != nil {
		log.Error(fmt.Sprintf("Error generating AWSConsoleURL. %s", err))
	}
	return url
}

func (a *Instance) Filter(filter filters.Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
	case "State":
		if a.State != nil && *a.State.Name == filter.Arguments[0] {
			matched = true
		}
	case "InstanceType":
		if a.InstanceType != nil && *a.InstanceType == filter.Arguments[0] {
			matched = true
		}
	case "HasPublicIPAddress":
		if b, err := filter.BoolValue(0); err == nil && b == (a.PublicIPAddress != nil) {
			matched = true
		}
	case "PublicIPAddress":
		if a.PublicIPAddress != nil && *a.PublicIPAddress == filter.Arguments[0] {
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
		for _, region := range filter.Arguments {
			if a.Region == reapable.Region(region) {
				matched = true
			}
		}
	case "NotRegion":
		// was this resource's region one of those in the NOT list
		regionSpecified := false
		for _, region := range filter.Arguments {
			if a.Region == reapable.Region(region) {
				regionSpecified = true
			}
		}
		if !regionSpecified {
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
	case "TagNotEqual":
		if a.Tag(filter.Arguments[0]) != filter.Arguments[1] {
			matched = true
		}
	case "ReaperState":
		if a.reaperState.State.String() == filter.Arguments[0] {
			matched = true
		}
	case "NotReaperState":
		if a.reaperState.State.String() != filter.Arguments[0] {
			matched = true
		}
	case "Named":
		if a.Name == filter.Arguments[0] {
			matched = true
		}
	case "NotNamed":
		if a.Name != filter.Arguments[0] {
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
	case "NotNameContains":
		if !strings.Contains(a.Name, filter.Arguments[0]) {
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

func (i *Instance) Start() (bool, error) {
	log.Notice("Starting Instance %s", i.ReapableDescriptionTiny())
	api := ec2.New(&aws.Config{Region: string(i.Region)})
	req := &ec2.StartInstancesInput{
		InstanceIDs: []*string{aws.String(string(i.ID))},
	}

	resp, err := api.StartInstances(req)

	if err != nil {
		return false, err
	}

	if len(resp.StartingInstances) != 1 {
		return false, fmt.Errorf("Instance %s could not be started.", i.ReapableDescriptionTiny())
	}

	return true, nil
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
