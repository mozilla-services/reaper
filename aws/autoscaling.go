package aws

import (
	"bytes"
	"fmt"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"

	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

type autoScalingGroupScalingSchedule struct {
	Enabled           bool
	ScaleDownString   string
	ScaleUpString     string
	previousScaleSize int64
	previousMinSize   int64
}

func (s *autoScalingGroupScalingSchedule) setSchedule(tag string) {
	// scalerTag format: cron format schedule (scale down),cron format schedule (scale up),previous scale time,previous desired size,previous min size
	splitTag := strings.Split(tag, ",")
	if len(splitTag) != 4 {
		log.Error("Invalid Autoscaler Tag format ", tag)
		return
	}
	prev, err := strconv.ParseInt(splitTag[2], 0, 64)
	if err != nil {
		log.Error("Invalid Autoscaler Tag format ", tag)
		log.Error(err.Error())
		return
	}
	min, err := strconv.ParseInt(splitTag[3], 0, 64)
	if err != nil {
		log.Error("Invalid Autoscaler Tag format ", tag)
		log.Error(err.Error())
		return
	}
	s.ScaleDownString = splitTag[0]
	s.ScaleUpString = splitTag[1]
	s.previousScaleSize = prev
	s.previousMinSize = min
	s.Enabled = true
}

// SaveSchedule is a method of the Scaler interface
func (a *AutoScalingGroup) SaveSchedule() {
	tagAutoScalingGroup(a.Region, a.ID, scalerTag, a.Scheduling.scheduleTag())
}

// SetScaleDownString is a method of the Scaler interface
func (a *AutoScalingGroup) SetScaleDownString(s string) {
	a.Scheduling.ScaleDownString = s
}

// SetScaleUpString is a method of the Scaler interface
func (a *AutoScalingGroup) SetScaleUpString(s string) {
	a.Scheduling.ScaleUpString = s
}

func (s *autoScalingGroupScalingSchedule) scheduleTag() string {
	return strings.Join([]string{
		// keep the same schedules
		s.ScaleDownString,
		s.ScaleUpString,
		strconv.FormatInt(s.previousScaleSize, 10),
		strconv.FormatInt(s.previousMinSize, 10),
	}, ",")
}

// AutoScalingGroup is a Reapable, Filterable
// embeds AWS API's autoscaling.Group
type AutoScalingGroup struct {
	Resource
	autoscaling.Group

	Scheduling autoScalingGroupScalingSchedule
	// autoscaling.Instance exposes minimal info
	Instances []reapable.ID
}

// NewAutoScalingGroup creates an AutoScalingGroup from the AWS API's autoscaling.Group
func NewAutoScalingGroup(region string, asg *autoscaling.Group) *AutoScalingGroup {
	a := AutoScalingGroup{
		Resource: Resource{
			Region: reapable.Region(region),
			ID:     reapable.ID(*asg.AutoScalingGroupName),
			Name:   *asg.AutoScalingGroupName,
			Tags:   make(map[string]string),
		},
		Group: *asg,
	}

	for _, instance := range asg.Instances {
		a.Instances = append(a.Instances, reapable.ID(*instance.InstanceId))
	}

	for _, tag := range asg.Tags {
		a.Resource.Tags[*tag.Key] = *tag.Value
	}

	if a.Tagged("aws:cloudformation:stack-name") {
		a.Dependency = true
		a.IsInCloudformation = true
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
		a.reaperState = state.NewState()
	}

	return &a
}

// ReapableEventText is part of the events.Reapable interface
func (a *AutoScalingGroup) ReapableEventText() (*bytes.Buffer, error) {
	return reapableEventText(a, reapableASGEventText)
}

// ReapableEventTextShort is part of the events.Reapable interface
func (a *AutoScalingGroup) ReapableEventTextShort() (*bytes.Buffer, error) {
	return reapableEventText(a, reapableASGEventTextShort)
}

// ReapableEventEmail is part of the events.Reapable interface
func (a *AutoScalingGroup) ReapableEventEmail() (owner mail.Address, subject string, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{ErrorText: fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}

	subject = fmt.Sprintf("AWS Resource %s is going to be Reaped!", a.ReapableDescriptionTiny())
	owner = *a.Owner()
	body, err = reapableEventHTML(a, reapableASGEventHTML)
	return
}

// ReapableEventEmailShort is part of the events.Reapable interface
func (a *AutoScalingGroup) ReapableEventEmailShort() (owner mail.Address, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{ErrorText: fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}
	owner = *a.Owner()
	body, err = reapableEventHTML(a, reapableASGEventHTMLShort)
	return
}

type autoScalingGroupEventData struct {
	Config                           *Config
	AutoScalingGroup                 *AutoScalingGroup
	TerminateLink                    string
	StopLink                         string
	ForceStopLink                    string
	WhitelistLink                    string
	IgnoreLink1                      string
	IgnoreLink3                      string
	IgnoreLink7                      string
	SchedulePacificBusinessHoursLink string
	ScheduleEasternBusinessHoursLink string
	ScheduleCESTBusinessHoursLink    string
}

func (a *AutoScalingGroup) getTemplateData() (interface{}, error) {
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
	forcestop, err := makeForceStopLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.APIURL)
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

	return &autoScalingGroupEventData{
		Config:                           config,
		AutoScalingGroup:                 a,
		TerminateLink:                    terminate,
		StopLink:                         stop,
		ForceStopLink:                    forcestop,
		WhitelistLink:                    whitelist,
		IgnoreLink1:                      ignore1,
		IgnoreLink3:                      ignore3,
		IgnoreLink7:                      ignore7,
		SchedulePacificBusinessHoursLink: schedulePacific,
		ScheduleEasternBusinessHoursLink: scheduleEastern,
		ScheduleCESTBusinessHoursLink:    scheduleCEST,
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
			<li><a href="{{ .SchedulePacificBusinessHoursLink}}">Schedule it to scale up and down with Pacific business hours</a></li>
			<li><a href="{{ .ScheduleEasternBusinessHoursLink}}">Schedule it to scale up and down with Eastern business hours</a></li>
			<li><a href="{{ .ScheduleCESTBusinessHoursLink}}">Schedule it to scale up and down with CEST business hours</a></li>
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
		Schedule it to scale up and down with <a href="{{ .SchedulePacificBusinessHoursLink}}">Pacific</a>,
		<a href="{{ .ScheduleEasternBusinessHoursLink}}">Eastern</a>, or
		<a href="{{ .ScheduleCESTBusinessHoursLink}}">CEST</a> business hours,
		<a href="{{ .TerminateLink }}">Terminate</a>,
		<a href="{{ .StopLink }}">Stop</a>,
		<a href="{{ .IgnoreLink1 }}">Ignore it for 1 more day</a>,
		<a href="{{ .IgnoreLink3 }}">3 days</a>,
		<a href="{{ .IgnoreLink7}}"> 7 days</a>,
		<a href="{{ .WhitelistLink }}">Whitelist</a> it.
	</p>
</body>
</html>
`

const reapableASGEventTextShort = `%%%
AutoScalingGroup [{{.AutoScalingGroup.ID}}]({{.AutoScalingGroup.AWSConsoleURL}}) in region: [{{.AutoScalingGroup.Region}}](https://{{.AutoScalingGroup.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.AutoScalingGroup.Region}}).{{if .AutoScalingGroup.Owned}} Owned by {{.AutoScalingGroup.Owner}}.\n{{end}}
Schedule this AutoScalingGroup to scale up and down with [Pacific]({{ .SchedulePacificBusinessHoursLink}}), [Eastern]({{ .ScheduleEasternBusinessHoursLink}}), or [CEST]({{ .ScheduleCESTBusinessHoursLink}}) business hours.\n
[Whitelist]({{ .WhitelistLink }}), [Scale to 0]({{ .StopLink }}), [ForceScale to 0]({{ .ForceStopLink }}), or [Terminate]({{ .TerminateLink }}) this AutoScalingGroup.
%%%`

const reapableASGEventText = `%%%
Reaper has discovered an AutoScalingGroup qualified as reapable: [{{.AutoScalingGroup.ID}}]({{.AutoScalingGroup.AWSConsoleURL}}) in region: [{{.AutoScalingGroup.Region}}](https://{{.AutoScalingGroup.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.AutoScalingGroup.Region}}).\n
{{if .AutoScalingGroup.Owned}}Owned by {{.AutoScalingGroup.Owner}}.\n{{end}}
{{ if .AutoScalingGroup.AWSConsoleURL}}{{.AutoScalingGroup.AWSConsoleURL}}\n{{end}}
[AWS Console URL]({{.AutoScalingGroup.AWSConsoleURL}})\n
Schedule this AutoScalingGroup to scale up and down with [Pacific]({{ .SchedulePacificBusinessHoursLink}}), [Eastern]({{ .ScheduleEasternBusinessHoursLink}}), or [CEST]({{ .ScheduleCESTBusinessHoursLink}}) business hours.\n
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

// Save is part of reapable.Saveable, which embedded in reapable.Reapable
func (a *AutoScalingGroup) Save(s *state.State) (bool, error) {
	return tagAutoScalingGroup(a.Region, a.ID, reaperTag, a.reaperState.String())
}

// Unsave is part of reapable.Saveable, which embedded in reapable.Reapable
func (a *AutoScalingGroup) Unsave() (bool, error) {
	log.Info("Unsaving %s", a.ReapableDescriptionTiny())
	return untagAutoScalingGroup(a.Region, a.ID, reaperTag)
}

func untagAutoScalingGroup(region reapable.Region, id reapable.ID, key string) (bool, error) {
	api := autoscaling.New(session.New(&aws.Config{Region: aws.String(string(region))}))
	deletereq := &autoscaling.DeleteTagsInput{
		Tags: []*autoscaling.Tag{
			&autoscaling.Tag{
				ResourceId:   aws.String(string(id)),
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
	api := autoscaling.New(session.New(&aws.Config{Region: aws.String(string(region))}))
	createreq := &autoscaling.CreateOrUpdateTagsInput{
		Tags: []*autoscaling.Tag{
			&autoscaling.Tag{
				ResourceId:        aws.String(string(id)),
				ResourceType:      aws.String("auto-scaling-group"),
				PropagateAtLaunch: aws.Bool(false),
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

// Filter is part of the filter.Filterable interface
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
		log.Error(fmt.Sprintf("No function %s could be found for filtering AutoScalingGroups.", filter.Function))
	}
	return matched
}

// AWSConsoleURL returns the url that can be used to access the resource on the AWS Console
func (a *AutoScalingGroup) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/autoscaling/home?region=%s#AutoScalingGroups:id=%s;view=details",
		a.Region.String(), a.Region.String(), url.QueryEscape(a.ID.String())))
	if err != nil {
		log.Error("Error generating AWSConsoleURL. ", err)
	}
	return url
}

// ScaleDown scales an AutoScalingGroup's DesiredCapacity down
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

// ScaleUp scales an AutoScalingGroup's DesiredCapacity up
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
	log.Info("Scaling AutoScalingGroup %s to size %d.", a.ReapableDescriptionTiny(), size)
	as := autoscaling.New(session.New(&aws.Config{Region: aws.String(string(a.Region))}))
	idString := a.ID.String()
	input := &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: &idString,
		DesiredCapacity:      &size,
		MinSize:              &minSize,
	}

	_, err := as.UpdateAutoScalingGroup(input)
	if err != nil {
		log.Error("could not update AutoScalingGroup ", a.ReapableDescriptionTiny())
		return false, err
	}
	return true, nil
}

// Terminate is a method of reapable.Terminable, which is embedded in reapable.Reapable
func (a *AutoScalingGroup) Terminate() (bool, error) {
	log.Info("Terminating AutoScalingGroup %s", a.ReapableDescriptionTiny())
	as := autoscaling.New(session.New(&aws.Config{Region: aws.String(string(a.Region))}))
	idString := a.ID.String()
	input := &autoscaling.DeleteAutoScalingGroupInput{
		AutoScalingGroupName: &idString,
	}
	_, err := as.DeleteAutoScalingGroup(input)
	if err != nil {
		log.Error("could not delete AutoScalingGroup ", a.ReapableDescriptionTiny())
		return false, err
	}
	return true, nil
}

// Whitelist is a method of reapable.Whitelistable, which is embedded in reapable.Reapable
func (a *AutoScalingGroup) Whitelist() (bool, error) {
	log.Info("Whitelisting AutoScalingGroup %s", a.ReapableDescriptionTiny())
	api := autoscaling.New(session.New(&aws.Config{Region: aws.String(string(a.Region))}))
	createreq := &autoscaling.CreateOrUpdateTagsInput{
		Tags: []*autoscaling.Tag{
			&autoscaling.Tag{
				ResourceId:        aws.String(string(a.ID)),
				ResourceType:      aws.String("auto-scaling-group"),
				PropagateAtLaunch: aws.Bool(false),
				Key:               aws.String(config.WhitelistTag),
				Value:             aws.String("true"),
			},
		},
	}
	_, err := api.CreateOrUpdateTags(createreq)
	return err == nil, err
}

// Stop is a method of reapable.Stoppable, which is embedded in reapable.Reapable
// Stop scales ASGs to 0
func (a *AutoScalingGroup) Stop() (bool, error) {
	// use existing min size
	return a.scaleToSize(0, *a.MinSize)
}

// ForceStop is a method of reapable.Stoppable, which is embedded in reapable.Reapable
// ForceStop force scales ASGs to 0
func (a *AutoScalingGroup) ForceStop() (bool, error) {
	// also set minsize to 0
	return a.scaleToSize(0, 0)
}
