package aws

import (
	"bytes"
	"fmt"
	htmlTemplate "html/template"
	"net/mail"

	"net/url"
	"os"

	textTemplate "text/template"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"

	"github.com/mostlygeek/reaper/filters"
	"github.com/mostlygeek/reaper/reapable"
	"github.com/mostlygeek/reaper/state"
)

type AutoScalingGroup struct {
	AWSResource

	// autoscaling.Instance exposes minimal info
	Instances []string

	AutoScalingGroupARN     string
	CreatedTime             time.Time
	MaxSize                 int64
	MinSize                 int64
	DesiredCapacity         int64
	LaunchConfigurationName string
}

func NewAutoScalingGroup(region string, asg *autoscaling.Group) *AutoScalingGroup {
	a := AutoScalingGroup{
		AWSResource: AWSResource{
			ID:          *asg.AutoScalingGroupName,
			Name:        *asg.AutoScalingGroupName,
			Region:      region,
			Tags:        make(map[string]string),
			reaperState: state.NewStateWithUntil(time.Now().Add(config.Notifications.FirstNotification.Duration)),
		},
		AutoScalingGroupARN:     *asg.AutoScalingGroupARN,
		CreatedTime:             *asg.CreatedTime,
		MaxSize:                 *asg.MaxSize,
		MinSize:                 *asg.MinSize,
		DesiredCapacity:         *asg.DesiredCapacity,
		LaunchConfigurationName: *asg.LaunchConfigurationName,
	}

	for i := 0; i < len(asg.Instances); i++ {
		a.Instances = append(a.Instances, *asg.Instances[i].InstanceID)
	}

	for i := 0; i < len(asg.Tags); i++ {
		a.Tags[*asg.Tags[i].Key] = *asg.Tags[i].Value
	}

	return &a
}

func (a *AutoScalingGroup) ReapableEventText() *bytes.Buffer {
	t := textTemplate.Must(textTemplate.New("reapable-asg").Parse(reapableASGEventText))
	buf := bytes.NewBuffer(nil)

	data, err := a.getTemplateData()
	if err != nil {
		log.Error(fmt.Sprintf("%s", err.Error()))
	}
	err = t.Execute(buf, data)
	if err != nil {
		log.Debug("Template generation error", err)
	}
	return buf
}

func (a *AutoScalingGroup) ReapableEventEmail() (owner mail.Address, subject string, body string, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error(fmt.Sprintf("eventHTML: %s", r))
		}
	}()

	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", a.ReapableDescription())}
		return
	}

	t := htmlTemplate.Must(htmlTemplate.New("reapable-asg").Parse(reapableASGEventHTML))
	buf := bytes.NewBuffer(nil)

	data, err := a.getTemplateData()
	err = t.Execute(buf, data)
	if err != nil {
		return
	}
	subject = fmt.Sprintf("AWS Resource %s is going to be Reaped!", a.ReapableDescription())
	owner = *a.Owner()
	body = buf.String()
	return
}

type AutoScalingGroupEventData struct {
	Config           *AWSConfig
	AutoScalingGroup *AutoScalingGroup
	TerminateLink    string
	StopLink         string
	ForceStopLink    string
	WhitelistLink    string
	IgnoreLink1      string
	IgnoreLink3      string
	IgnoreLink7      string
}

func (a *AutoScalingGroup) getTemplateData() (*AutoScalingGroupEventData, error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error(fmt.Sprintf("getTemplateData: %s", r))
			os.Exit(1)
		}
	}()

	ignore1, err := MakeIgnoreLink(config.HTTP.TokenSecret, config.HTTP.ApiURL, a.Region, a.ID, time.Duration(1*24*time.Hour))
	ignore3, err := MakeIgnoreLink(config.HTTP.TokenSecret, config.HTTP.ApiURL, a.Region, a.ID, time.Duration(3*24*time.Hour))
	ignore7, err := MakeIgnoreLink(config.HTTP.TokenSecret, config.HTTP.ApiURL, a.Region, a.ID, time.Duration(7*24*time.Hour))
	terminate, err := MakeTerminateLink(config.HTTP.TokenSecret, config.HTTP.ApiURL, a.Region, a.ID)
	stop, err := MakeStopLink(config.HTTP.TokenSecret, config.HTTP.ApiURL, a.Region, a.ID)
	forcestop, err := MakeForceStopLink(config.HTTP.TokenSecret, config.HTTP.ApiURL, a.Region, a.ID)
	whitelist, err := MakeWhitelistLink(config.HTTP.TokenSecret, config.HTTP.ApiURL, a.Region, a.ID)

	if err != nil {
		return nil, err
	}

	return &AutoScalingGroupEventData{
		Config:           config,
		AutoScalingGroup: a,
		TerminateLink:    terminate,
		StopLink:         stop,
		ForceStopLink:    forcestop,
		WhitelistLink:    whitelist,
		IgnoreLink1:      ignore1,
		IgnoreLink3:      ignore3,
		IgnoreLink7:      ignore7,
	}, nil
}

const reapableASGEventHTML = `
<html>
<body>
	<p>Your AWS Resource <a href="{{ .AutoScalingGroup.AWSConsoleURL }}">{{ if .AutoScalingGroup.Name }}"{{.AutoScalingGroup.Name}}" {{ end }} in {{.AutoScalingGroup.Region}}</a> is scheduled to be terminated.</p>

	<p>
		You can ignore this message and your AutoScalingGroup will be automatically
		terminated after <strong>{{.AutoScalingGroup.ReaperState.Until}}</strong>.
	</p>

	<p>
		You may also choose to:
		<ul>
			<li><a href="{{ .TerminateLink }}">Terminate it now</a></li>
			<li><a href="{{ .StopLink }}">Scale it to 0</a></li>
			<li><a href="{{ .ForceStopLink }}">ForceScale it to 0</a></li>
			<li><a href="{{ .IgnoreLink1 }}">Ignore it for 1 more day</a></li>
			<li><a href="{{ .IgnoreLink3 }}">Ignore it for 3 more days</a></li>
			<li><a href="{{ .IgnoreLink7}}">Ignore it for 7 more days</a></li>
		</ul>
	</p>

	<p>
		If you want the Reaper to ignore this AutoScalingGroup tag it with {{ .Config.WhitelistTag }} with any value, or click <a href="{{ .WhitelistLink }}">here</a>.
	</p>
</body>
</html>
`

const reapableASGEventText = `%%%
Reaper has discovered an AutoScalingGroup qualified as reapable: [{{.AutoScalingGroup.ID}}]({{.AutoScalingGroup.AWSConsoleURL}}) in region: [{{.AutoScalingGroup.Region}}](https://{{.AutoScalingGroup.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.AutoScalingGroup.Region}}).\n
{{if .AutoScalingGroup.Owned}}Owned by {{.AutoScalingGroup.Owner}}.\n{{end}}
{{ if .AutoScalingGroup.AWSConsoleURL}}{{.AutoScalingGroup.AWSConsoleURL}}\n{{end}}
[AWS Console URL]({{.AutoScalingGroup.AWSConsoleURL}})\n
[Whitelist]({{ .WhitelistLink }}) this AutoScalingGroup.
[Scale to 0]({{ .StopLink }}) this AutoScalingGroup.
[ForceScale to 0]({{ .ForceStopLink }}) this AutoScalingGroup.
[Terminate]({{ .TerminateLink }}) this AutoScalingGroup.
%%%`

func (a *AutoScalingGroup) SizeGreaterThanOrEqualTo(size int64) bool {
	return a.DesiredCapacity >= size
}

func (a *AutoScalingGroup) SizeLessThanOrEqualTo(size int64) bool {
	return a.DesiredCapacity <= size
}

func (a *AutoScalingGroup) SizeEqualTo(size int64) bool {
	return a.DesiredCapacity == size
}

func (a *AutoScalingGroup) SizeLessThan(size int64) bool {
	return a.DesiredCapacity < size
}

func (a *AutoScalingGroup) SizeGreaterThan(size int64) bool {
	return a.DesiredCapacity <= size
}

// method for reapable -> overrides promoted AWSResource method of same name?
func (a *AutoScalingGroup) Save(s *state.State) (bool, error) {
	return a.tagReaperState(a.Region, a.ID, a.ReaperState())
}

func (a *AutoScalingGroup) tagReaperState(region, id string, newState *state.State) (bool, error) {
	api := autoscaling.New(&aws.Config{Region: region})
	createreq := &autoscaling.CreateOrUpdateTagsInput{
		Tags: []*autoscaling.Tag{
			&autoscaling.Tag{
				ResourceID:        aws.String(id),
				ResourceType:      aws.String("auto-scaling-group"),
				PropagateAtLaunch: aws.Boolean(false),
				Key:               aws.String(reaperTag),
				Value:             aws.String(newState.String()),
			},
		},
	}

	_, err := api.CreateOrUpdateTags(createreq)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (a *AutoScalingGroup) Filter(filter filters.Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
	case "SizeGreaterThan":
		if i, err := filter.Int64Value(0); err == nil && a.SizeGreaterThan(i) {
			matched = true
		}
	case "SizeLessThan":
		if i, err := filter.Int64Value(0); err == nil && a.SizeLessThan(i) {
			matched = true
		}
	case "SizeEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.SizeEqualTo(i) {
			matched = true
		}
	case "SizeLessThanOrEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.SizeLessThanOrEqualTo(i) {
			matched = true
		}
	case "SizeGreaterThanOrEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.SizeGreaterThanOrEqualTo(i) {
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
	default:
		log.Error(fmt.Sprintf("No function %s could be found for filtering AutoScalingGroups.", filter.Function))
	}
	return matched
}

func (a *AutoScalingGroup) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/autoscaling/home?region=%s#AutoScalingGroups:id=%s;view=details",
		a.Region, a.Region, url.QueryEscape(a.ID)))
	if err != nil {
		log.Error(fmt.Sprintf("Error generating AWSConsoleURL. %s", err))
	}
	return url
}

func (a *AutoScalingGroup) scaleToSize(force bool, size int64) (bool, error) {
	log.Debug("Stopping ASG %s in region %s", a.ID, a.Region)
	as := autoscaling.New(&aws.Config{Region: a.Region})

	input := &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: &a.ID,
		DesiredCapacity:      &size,
	}

	if force {
		input.MinSize = &size
	}

	_, err := as.UpdateAutoScalingGroup(input)
	if err != nil {
		log.Error(fmt.Sprintf("could not update ASG %s in region %s", a.ID, a.Region))
		return false, err
	}
	return true, nil
}

// TODO
func (a *AutoScalingGroup) Terminate() (bool, error) {
	log.Debug("Terminating ASG %s in region %s.", a.ID, a.Region)
	return false, nil
}

// Stop scales ASGs to 0
func (a *AutoScalingGroup) Stop() (bool, error) {
	// force -> false
	return a.scaleToSize(false, 0)
}

// ForceStop force scales ASGs to 0
func (a *AutoScalingGroup) ForceStop() (bool, error) {
	// force -> true
	return a.scaleToSize(true, 0)
}
