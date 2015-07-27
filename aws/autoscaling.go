package aws

import (
	"bytes"
	"fmt"
	htmlTemplate "html/template"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	textTemplate "text/template"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"

	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

type AutoScalingGroupScalingSchedule struct {
	enabled           bool
	scaleDownString   string
	scaleUpString     string
	previousScaleSize int64
	previousMinSize   int64
}

func (s *AutoScalingGroupScalingSchedule) setSchedule(tag string) {
	// scalerTag format: cron format schedule (scale down),cron format schedule (scale up),previous scale time,previous desired size,previous min size
	splitTag := strings.Split(tag, ",")
	if len(splitTag) != 4 {
		log.Error("Invalid Autoscaler Tag format %s", tag)
	} else {
		prev, err := strconv.ParseInt(splitTag[2], 0, 64)
		if err != nil {
			log.Error("Invalid Autoscaler Tag format %s", tag)
			log.Error(err.Error())
			return
		}
		min, err := strconv.ParseInt(splitTag[3], 0, 64)
		if err != nil {
			log.Error("Invalid Autoscaler Tag format %s", tag)
			log.Error(err.Error())
			return
		}
		s.scaleDownString = splitTag[0]
		s.scaleUpString = splitTag[1]
		s.previousScaleSize = prev
		s.previousMinSize = min
		s.enabled = true
	}
}

func (s AutoScalingGroupScalingSchedule) scheduleTag() string {
	return strings.Join([]string{
		// keep the same schedules
		s.scaleDownString,
		s.scaleUpString,
		strconv.FormatInt(s.previousScaleSize, 10),
		strconv.FormatInt(s.previousMinSize, 10),
	}, ",")
}

type AutoScalingGroup struct {
	AWSResource
	autoscaling.Group

	Scheduling AutoScalingGroupScalingSchedule
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

	// autoscaler boilerplate
	if a.Tagged(scalerTag) {
		a.Scheduling.setSchedule(a.Tag(scalerTag))
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

func (a *AutoScalingGroup) ReapableEventEmail() (owner mail.Address, subject string, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}

	subject = fmt.Sprintf("AWS Resource %s is going to be Reaped!", a.ReapableDescriptionTiny())
	owner = *a.Owner()
	body = a.reapableEventHTML(reapableASGEventHTML)
	return
}

func (a *AutoScalingGroup) ReapableEventEmailShort() (owner mail.Address, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}
	owner = *a.Owner()
	body = a.reapableEventHTML(reapableASGEventHTMLShort)
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
	return tagAutoScalingGroup(a.Region, a.ID, reaperTag, a.reaperState.String())
}

// method for reapable -> overrides promoted AWSResource method of same name?
func (a *AutoScalingGroup) Unsave() (bool, error) {
	log.Notice("Unsaving %s", a.ReapableDescriptionTiny())
	return untagAutoScalingGroup(a.Region, a.ID, reaperTag)
}

func untagAutoScalingGroup(region reapable.Region, id reapable.ID, key string) (bool, error) {
	api := autoscaling.New(&aws.Config{Region: string(region)})
	deletereq := &autoscaling.DeleteTagsInput{
		Tags: []*autoscaling.Tag{
			&autoscaling.Tag{
				ResourceID:   aws.String(string(id)),
				ResourceType: aws.String("auto-scaling-group"),
				Key:          aws.String(key),
			},
		},
	}

	_, err := api.DeleteTags(deletereq)
	if err != nil {
		return false, err
	}

	return true, nil
}

func tagAutoScalingGroup(region reapable.Region, id reapable.ID, key, value string) (bool, error) {
	log.Info("Tagging AutoScalingGroup %s in %s with %s:%s", region.String(), id.String(), key, value)
	api := autoscaling.New(&aws.Config{Region: string(region)})
	createreq := &autoscaling.CreateOrUpdateTagsInput{
		Tags: []*autoscaling.Tag{
			&autoscaling.Tag{
				ResourceID:        aws.String(string(id)),
				ResourceType:      aws.String("auto-scaling-group"),
				PropagateAtLaunch: aws.Boolean(false),
				Key:               aws.String(key),
				Value:             aws.String(value),
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
		a.Region.String(), a.Region.String(), url.QueryEscape(a.ID.String())))
	if err != nil {
		log.Error(fmt.Sprintf("Error generating AWSConsoleURL. %s", err))
	}
	return url
}

// Scaler interface
func (a *AutoScalingGroup) SchedulingEnabled() bool {
	return a.Scheduling.enabled
}

func (a *AutoScalingGroup) ScaleDownSchedule() string {
	return a.Scheduling.scaleDownString
}

func (a *AutoScalingGroup) ScaleUpSchedule() string {
	return a.Scheduling.scaleUpString
}

func (a *AutoScalingGroup) ScaleDown() {
	// default = 0
	var size int64
	// change desired and min to size
	if *a.DesiredCapacity > size {
		ok, err := a.scaleToSize(size, size)
		if ok && err == nil {
			a.Scheduling.previousScaleSize = *a.DesiredCapacity
			a.Scheduling.previousMinSize = *a.MinSize
			// change current local value so that we don't repeat
			*a.DesiredCapacity = size
		}
	}
}

func (a *AutoScalingGroup) ScaleUp() {
	if a.Scheduling.previousScaleSize > *a.DesiredCapacity {
		// change desired and min to previous values
		ok, err := a.scaleToSize(a.Scheduling.previousScaleSize, a.Scheduling.previousMinSize)
		if ok && err == nil {
			// change current local values so that we don't repeat
			*a.DesiredCapacity = a.Scheduling.previousScaleSize
			*a.MinSize = a.Scheduling.previousMinSize
		}
	}
}

func (a *AutoScalingGroup) scaleToSize(size int64, minSize int64) (bool, error) {
	log.Notice("Scaling AutoScalingGroup %s to size %d.", a.ReapableDescriptionTiny(), size)
	as := autoscaling.New(&aws.Config{Region: a.Region.String()})
	idString := a.ID.String()
	input := &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: &idString,
		DesiredCapacity:      &size,
		MinSize:              &minSize,
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
	as := autoscaling.New(&aws.Config{Region: a.Region.String()})
	idString := a.ID.String()
	input := &autoscaling.DeleteAutoScalingGroupInput{
		AutoScalingGroupName: &idString,
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
	// use existing min size
	return a.scaleToSize(0, *a.MinSize)
}

// ForceStop force scales ASGs to 0
func (a *AutoScalingGroup) ForceStop() (bool, error) {
	// also set minsize to 0
	return a.scaleToSize(0, 0)
}
