package aws

import (
	"bytes"
	"fmt"
	htmlTemplate "html/template"
	"net/mail"
	"net/url"
	textTemplate "text/template"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"

	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

type AutoScalingGroup struct {
	AWSResource
	autoscaling.Group

	// autoscaling.Instance exposes minimal info
	Instances []reapable.ID
}

func NewAutoScalingGroup(region string, asg *autoscaling.Group) *AutoScalingGroup {
	a := AutoScalingGroup{
		AWSResource: AWSResource{
			Region: reapable.Region(region),
			ID:     reapable.ID(*asg.AutoScalingGroupName),
			Name:   *asg.AutoScalingGroupName,
			Tags:   make(map[string]string),
		},
		Group: *asg,
	}

	for i := 0; i < len(asg.Instances); i++ {
		a.Instances = append(a.Instances, reapable.ID(*asg.Instances[i].InstanceID))
	}

	for i := 0; i < len(asg.Tags); i++ {
		a.AWSResource.Tags[*asg.Tags[i].Key] = *asg.Tags[i].Value
	}

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

func (a *AutoScalingGroup) reapableEventHTML(text string) *bytes.Buffer {
	t := htmlTemplate.Must(htmlTemplate.New("reapable").Parse(text))
	buf := bytes.NewBuffer(nil)

	data, err := a.getTemplateData()
	err = t.Execute(buf, data)
	if err != nil {
		log.Debug(fmt.Sprintf("Template generation error: %s", err))
	}
	return buf
}

func (a *AutoScalingGroup) reapableEventText(text string) *bytes.Buffer {
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

func (a *AutoScalingGroup) ReapableEventText() *bytes.Buffer {
	return a.reapableEventText(reapableASGEventText)
}

func (a *AutoScalingGroup) ReapableEventTextShort() *bytes.Buffer {
	return a.reapableEventText(reapableASGEventTextShort)
}

func (a *AutoScalingGroup) ReapableEventEmail() (owner mail.Address, subject string, body string, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}

	subject = fmt.Sprintf("AWS Resource %s is going to be Reaped!", a.ReapableDescriptionTiny())
	owner = *a.Owner()
	body = a.reapableEventHTML(reapableASGEventHTML).String()
	return
}

func (a *AutoScalingGroup) ReapableEventEmailShort() (owner mail.Address, body string, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}
	owner = *a.Owner()
	body = a.reapableEventHTML(reapableASGEventHTMLShort).String()
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
	ignore1, err := MakeIgnoreLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL, time.Duration(1*24*time.Hour))
	ignore3, err := MakeIgnoreLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL, time.Duration(3*24*time.Hour))
	ignore7, err := MakeIgnoreLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL, time.Duration(7*24*time.Hour))
	terminate, err := MakeTerminateLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL)
	stop, err := MakeStopLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL)
	forcestop, err := MakeForceStopLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL)
	whitelist, err := MakeWhitelistLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL)

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
	<p>AutoScalingGroup <a href="{{ .AutoScalingGroup.AWSConsoleURL }}">{{ if .AutoScalingGroup.Name }}"{{.AutoScalingGroup.Name}}" {{ end }} in {{.AutoScalingGroup.Region}}</a> is scheduled to be terminated.</p>

	<p>
		You can ignore this message and your AutoScalingGroup will advance to the next state after <strong>{{.AutoScalingGroup.ReaperState.Until}}</strong>. If you do not take action it will be terminated!
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

const reapableASGEventHTMLShort = `
<html>
<body>
	<p>AutoScalingGroup <a href="{{ .AutoScalingGroup.AWSConsoleURL }}">{{ if .AutoScalingGroup.Name }}"{{.AutoScalingGroup.Name}}" {{ end }}</a> in {{.AutoScalingGroup.Region}}</a> is scheduled to be terminated after <strong>{{.AutoScalingGroup.ReaperState.Until}}</strong>.
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

const reapableASGEventTextShort = `%%%
AutoScalingGroup [{{.AutoScalingGroup.ID}}]({{.AutoScalingGroup.AWSConsoleURL}}) in region: [{{.AutoScalingGroup.Region}}](https://{{.AutoScalingGroup.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.AutoScalingGroup.Region}}).{{if .AutoScalingGroup.Owned}} Owned by {{.AutoScalingGroup.Owner}}.\n{{end}}
[Whitelist]({{ .WhitelistLink }}), [Scale to 0]({{ .StopLink }}), [ForceScale to 0]({{ .ForceStopLink }}), or [Terminate]({{ .TerminateLink }}) this AutoScalingGroup.
%%%`

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

func (a *AutoScalingGroup) sizeGreaterThanOrEqualTo(size int64) bool {
	if a.DesiredCapacity != nil {
		return *a.DesiredCapacity >= size
	}
	return false
}

func (a *AutoScalingGroup) sizeLessThanOrEqualTo(size int64) bool {
	if a.DesiredCapacity != nil {
		return *a.DesiredCapacity <= size
	}
	return false
}

func (a *AutoScalingGroup) sizeEqualTo(size int64) bool {
	if a.DesiredCapacity != nil {
		return *a.DesiredCapacity == size
	}
	return false
}

func (a *AutoScalingGroup) sizeLessThan(size int64) bool {
	if a.DesiredCapacity != nil {
		return *a.DesiredCapacity < size
	}
	return false
}

func (a *AutoScalingGroup) sizeGreaterThan(size int64) bool {
	if a.DesiredCapacity != nil {
		return *a.DesiredCapacity > size
	}
	return false
}

// method for reapable -> overrides promoted AWSResource method of same name?
func (a *AutoScalingGroup) Save(s *state.State) (bool, error) {
	return a.tagReaperState(a.Region, a.ID, a.ReaperState())
}

// method for reapable -> overrides promoted AWSResource method of same name?
func (a *AutoScalingGroup) Unsave() (bool, error) {
	log.Notice("Unsaving %s", a.ReapableDescriptionTiny())
	return a.untagReaperState(a.Region, a.ID, a.ReaperState())
}

func (a *AutoScalingGroup) untagReaperState(region reapable.Region, id reapable.ID, newState *state.State) (bool, error) {
	api := autoscaling.New(&aws.Config{Region: string(region)})
	deletereq := &autoscaling.DeleteTagsInput{
		Tags: []*autoscaling.Tag{
			&autoscaling.Tag{
				ResourceID:   aws.String(string(id)),
				ResourceType: aws.String("auto-scaling-group"),
				Key:          aws.String(reaperTag),
			},
		},
	}

	_, err := api.DeleteTags(deletereq)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (a *AutoScalingGroup) tagReaperState(region reapable.Region, id reapable.ID, newState *state.State) (bool, error) {
	api := autoscaling.New(&aws.Config{Region: string(region)})
	createreq := &autoscaling.CreateOrUpdateTagsInput{
		Tags: []*autoscaling.Tag{
			&autoscaling.Tag{
				ResourceID:        aws.String(string(id)),
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
		if i, err := filter.Int64Value(0); err == nil && a.sizeGreaterThan(i) {
			matched = true
		}
	case "SizeLessThan":
		if i, err := filter.Int64Value(0); err == nil && a.sizeLessThan(i) {
			matched = true
		}
	case "SizeEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.sizeEqualTo(i) {
			matched = true
		}
	case "SizeLessThanOrEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.sizeLessThanOrEqualTo(i) {
			matched = true
		}
	case "SizeGreaterThanOrEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.sizeGreaterThanOrEqualTo(i) {
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
	case "CreatedTimeInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && a.CreatedTime != nil && time.Since(*a.CreatedTime) < d {
			matched = true
		}
	case "CreatedTimeNotInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && a.CreatedTime != nil && time.Since(*a.CreatedTime) > d {
			matched = true
		}
	case "InCloudformation":
		if b, err := filter.BoolValue(0); err == nil && a.IsInCloudformation == b {
			matched = true
		}
	case "IsDependency":
		if b, err := filter.BoolValue(0); err == nil && a.Dependency == b {
			matched = true
		}
	default:
		log.Error(fmt.Sprintf("No function %s could be found for filtering AutoScalingGroups.", filter.Function))
	}
	return matched
}

func (a *AutoScalingGroup) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/autoscaling/home?region=%s#AutoScalingGroups:id=%s;view=details",
		string(a.Region), string(a.Region), url.QueryEscape(string(a.ID))))
	if err != nil {
		log.Error(fmt.Sprintf("Error generating AWSConsoleURL. %s", err))
	}
	return url
}

func (a *AutoScalingGroup) scaleToSize(force bool, size int64) (bool, error) {
	log.Notice("Scaling AutoScalingGroup to size %d %s.", size, a.ReapableDescriptionTiny())
	as := autoscaling.New(&aws.Config{Region: string(a.Region)})

	// ugh this is stupid
	stringID := string(a.ID)

	input := &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: &stringID,
		DesiredCapacity:      &size,
	}

	if force {
		input.MinSize = &size
	}

	_, err := as.UpdateAutoScalingGroup(input)
	if err != nil {
		log.Error(fmt.Sprintf("could not update AutoScalingGroup %s", a.ReapableDescriptionTiny()))
		return false, err
	}
	return true, nil
}

func (a *AutoScalingGroup) Terminate() (bool, error) {
	log.Notice("Terminating AutoScalingGroup %s", a.ReapableDescriptionTiny())
	as := autoscaling.New(&aws.Config{Region: string(a.Region)})

	// ugh this is stupid
	stringID := string(a.ID)

	input := &autoscaling.DeleteAutoScalingGroupInput{
		AutoScalingGroupName: &stringID,
	}
	_, err := as.DeleteAutoScalingGroup(input)
	if err != nil {
		log.Error(fmt.Sprintf("could not delete AutoScalingGroup %s", a.ReapableDescriptionTiny()))
		return false, err
	}
	return false, nil
}

func (a *AutoScalingGroup) Whitelist() (bool, error) {
	log.Notice("Whitelisting AutoScalingGroup %s", a.ReapableDescriptionTiny())
	api := autoscaling.New(&aws.Config{Region: string(a.Region)})
	createreq := &autoscaling.CreateOrUpdateTagsInput{
		Tags: []*autoscaling.Tag{
			&autoscaling.Tag{
				ResourceID:        aws.String(string(a.ID)),
				ResourceType:      aws.String("auto-scaling-group"),
				PropagateAtLaunch: aws.Boolean(false),
				Key:               aws.String(config.WhitelistTag),
				Value:             aws.String("true"),
			},
		},
	}
	_, err := api.CreateOrUpdateTags(createreq)
	return err == nil, err
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
