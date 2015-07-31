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

type Volume struct {
	AWSResource
	ec2.Volume

	AttachedInstanceIDs []string
}

func NewVolume(region string, vol *ec2.Volume) *Volume {
	a := Volume{
		AWSResource: AWSResource{
			Region: reapable.Region(region),
			ID:     reapable.ID(*vol.VolumeID),
			Name:   *vol.VolumeID,
			Tags:   make(map[string]string),
		},
		Volume: *vol,
	}

	for i := 0; i < len(vol.Tags); i++ {
		a.AWSResource.Tags[*vol.Tags[i].Key] = *vol.Tags[i].Value
	}

	for _, attachment := range vol.Attachments {
		if attachment.InstanceID != nil {
			a.AttachedInstanceIDs = append(a.AttachedInstanceIDs, *attachment.InstanceID)
		}
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

func (a *Volume) reapableEventHTML(text string) *bytes.Buffer {
	t := htmlTemplate.Must(htmlTemplate.New("reapable").Parse(text))
	buf := bytes.NewBuffer(nil)

	data, err := a.getTemplateData()
	err = t.Execute(buf, data)
	if err != nil {
		log.Debug(fmt.Sprintf("Template generation error: %s", err))
	}
	return buf
}

func (a *Volume) reapableEventText(text string) *bytes.Buffer {
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

func (a *Volume) ReapableEventText() *bytes.Buffer {
	return a.reapableEventText(reapableVolumeEventText)
}

func (a *Volume) ReapableEventTextShort() *bytes.Buffer {
	return a.reapableEventText(reapableVolumeEventTextShort)
}

func (a *Volume) ReapableEventEmail() (owner mail.Address, subject string, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}

	subject = fmt.Sprintf("AWS Resource %s is going to be Reaped!", a.ReapableDescriptionTiny())
	owner = *a.Owner()
	body = a.reapableEventHTML(reapableVolumeEventHTML)
	return
}

func (a *Volume) ReapableEventEmailShort() (owner mail.Address, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}
	owner = *a.Owner()
	body = a.reapableEventHTML(reapableVolumeEventHTMLShort)
	return
}

type VolumeEventData struct {
	Config                           *AWSConfig
	Volume                           *Volume
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

func (a *Volume) getTemplateData() (*VolumeEventData, error) {
	ignore1, err := MakeIgnoreLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL, time.Duration(1*24*time.Hour))
	if err != nil {
		return nil, err
	}
	ignore3, err := MakeIgnoreLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL, time.Duration(3*24*time.Hour))
	if err != nil {
		return nil, err
	}
	ignore7, err := MakeIgnoreLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL, time.Duration(7*24*time.Hour))
	if err != nil {
		return nil, err
	}
	terminate, err := MakeTerminateLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL)
	if err != nil {
		return nil, err
	}
	stop, err := MakeStopLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL)
	if err != nil {
		return nil, err
	}
	forcestop, err := MakeForceStopLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL)
	if err != nil {
		return nil, err
	}
	whitelist, err := MakeWhitelistLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.ApiURL)
	if err != nil {
		return nil, err
	}

	return &VolumeEventData{
		Config:        config,
		Volume:        a,
		TerminateLink: terminate,
		StopLink:      stop,
		ForceStopLink: forcestop,
		WhitelistLink: whitelist,
		IgnoreLink1:   ignore1,
		IgnoreLink3:   ignore3,
		IgnoreLink7:   ignore7,
	}, nil
}

const reapableVolumeEventHTML = `
<html>
<body>
	<p>Volume <a href="{{ .Volume.AWSConsoleURL }}">{{ if .Volume.Name }}"{{.Volume.Name}}" {{ end }} in {{.Volume.Region}}</a> is scheduled to be terminated.</p>

	<p>
		You can ignore this message and your Volume will advance to the next state after <strong>{{.Volume.ReaperState.Until}}</strong>. If you do not take action it will be terminated!
	</p>

	<p>
		You may also choose to:
		<ul>
			<li><a href="{{ .TerminateLink }}">Terminate it now</a></li>
			<li><a href="{{ .IgnoreLink1 }}">Ignore it for 1 more day</a></li>
			<li><a href="{{ .IgnoreLink3 }}">Ignore it for 3 more days</a></li>
			<li><a href="{{ .IgnoreLink7}}">Ignore it for 7 more days</a></li>
		</ul>
	</p>

	<p>
		If you want the Reaper to ignore this Volume tag it with {{ .Config.WhitelistTag }} with any value, or click <a href="{{ .WhitelistLink }}">here</a>.
	</p>
</body>
</html>
`

const reapableVolumeEventHTMLShort = `
<html>
<body>
	<p>Volume <a href="{{ .Volume.AWSConsoleURL }}">{{ if .Volume.Name }}"{{.Volume.Name}}" {{ end }}</a> in {{.Volume.Region}}</a> is scheduled to be terminated after <strong>{{.Volume.ReaperState.Until}}</strong>.
		<br />
		<a href="{{ .TerminateLink }}">Terminate</a>,
		<a href="{{ .IgnoreLink1 }}">Ignore it for 1 more day</a>,
		<a href="{{ .IgnoreLink3 }}">3 days</a>,
		<a href="{{ .IgnoreLink7}}"> 7 days</a>,
		<a href="{{ .WhitelistLink }}">Whitelist</a> it.
	</p>
</body>
</html>
`

const reapableVolumeEventTextShort = `%%%
Volume [{{.Volume.ID}}]({{.Volume.AWSConsoleURL}}) in region: [{{.Volume.Region}}](https://{{.Volume.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.Volume.Region}}).{{if .Volume.Owned}} Owned by {{.Volume.Owner}}.\n{{end}}
[Whitelist]({{ .WhitelistLink }}) or [Terminate]({{ .TerminateLink }}) this Volume.
%%%`

const reapableVolumeEventText = `%%%
Reaper has discovered an Volume qualified as reapable: [{{.Volume.ID}}]({{.Volume.AWSConsoleURL}}) in region: [{{.Volume.Region}}](https://{{.Volume.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.Volume.Region}}).\n
{{if .Volume.Owned}}Owned by {{.Volume.Owner}}.\n{{end}}
{{ if .Volume.AWSConsoleURL}}{{.Volume.AWSConsoleURL}}\n{{end}}
[AWS Console URL]({{.Volume.AWSConsoleURL}})\n
[Whitelist]({{ .WhitelistLink }}) this Volume.
[Terminate]({{ .TerminateLink }}) this Volume.
%%%`

func (a *Volume) sizeGreaterThanOrEqualTo(size int64) bool {
	if a.Size != nil {
		return *a.Size >= size
	}
	return false
}

func (a *Volume) sizeLessThanOrEqualTo(size int64) bool {
	if a.Size != nil {
		return *a.Size <= size
	}
	return false
}

func (a *Volume) sizeEqualTo(size int64) bool {
	if a.Size != nil {
		return *a.Size == size
	}
	return false
}

func (a *Volume) sizeLessThan(size int64) bool {
	if a.Size != nil {
		return *a.Size < size
	}
	return false
}

func (a *Volume) sizeGreaterThan(size int64) bool {
	if a.Size != nil {
		return *a.Size > size
	}
	return false
}

// method for reapable -> overrides promoted AWSResource method of same name?
func (a *Volume) Save(s *state.State) (bool, error) {
	return tag(a.Region.String(), a.ID.String(), reaperTag, a.reaperState.String())
}

// method for reapable -> overrides promoted AWSResource method of same name?
func (a *Volume) Unsave() (bool, error) {
	log.Notice("Unsaving %s", a.ReapableDescriptionTiny())
	return untag(a.Region.String(), a.ID.String(), reaperTag)
}

func (a *Volume) Filter(filter filters.Filter) bool {
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
	case "CreatedInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && a.CreateTime != nil && time.Since(*a.CreateTime) < d {
			matched = true
		}
	case "CreatedNotInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && a.CreateTime != nil && time.Since(*a.CreateTime) > d {
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
	case "NameContains":
		if strings.Contains(a.Name, filter.Arguments[0]) {
			matched = true
		}
	case "State":
		// one of:
		// creating
		// available
		// in-use
		// deleting
		// deleted
		// error

		if a.State != nil && *a.State == filter.Arguments[0] {
			matched = true
		}
	case "AttachmentState":
		// one of:
		// attaching
		// attached
		// detaching
		// detached

		// I _think_ that the size of Attachments is only 0 or 1
		if len(a.Attachments) > 0 && *a.Attachments[0].State == filter.Arguments[0] {
			matched = true
		} else if len(a.Attachments) == 0 && "detached" == filter.Arguments[0] {
			matched = true
		}
	default:
		log.Error(fmt.Sprintf("No function %s could be found for filtering Volumes.", filter.Function))
	}
	return matched
}

func (a *Volume) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/v2/home?region=%s#Volumes:volumeId=%s",
		a.Region.String(), a.Region.String(), url.QueryEscape(a.ID.String())))
	if err != nil {
		log.Error(fmt.Sprintf("Error generating AWSConsoleURL. %s", err))
	}
	return url
}

func (a *Volume) Terminate() (bool, error) {
	log.Notice("Terminating Volume %s", a.ReapableDescriptionTiny())
	api := ec2.New(&aws.Config{Region: a.Region.String()})
	idString := a.ID.String()
	input := &ec2.DeleteVolumeInput{
		VolumeID: &idString,
	}
	_, err := api.DeleteVolume(input)
	if err != nil {
		log.Error(fmt.Sprintf("could not delete Volume %s", a.ReapableDescriptionTiny()))
		return false, err
	}
	return true, nil
}

// noop
func (a *Volume) Stop() (bool, error) {
	// use existing min size
	return false, nil
}

// noop
func (a *Volume) ForceStop() (bool, error) {
	return false, nil
}
