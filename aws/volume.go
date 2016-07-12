package aws

import (
	"bytes"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

// Volume is a is a Reapable, Filterable
// embeds AWS API's ec2.Volume
type Volume struct {
	Resource
	ec2.Volume

	AttachedInstanceIDs []string
}

// NewVolume creates an Volume from the AWS API's ec2.Volume
func NewVolume(region string, vol *ec2.Volume) *Volume {
	a := Volume{
		Resource: Resource{
			Region: reapable.Region(region),
			ID:     reapable.ID(*vol.VolumeID),
			Name:   *vol.VolumeID,
			Tags:   make(map[string]string),
		},
		Volume: *vol,
	}

	for _, tag := range vol.Tags {
		a.Resource.Tags[*tag.Key] = *tag.Value
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

// ReapableEventText is part of the events.Reapable interface
func (a *Volume) ReapableEventText() (*bytes.Buffer, error) {
	return reapableEventText(a, reapableVolumeEventText)
}

// ReapableEventTextShort is part of the events.Reapable interface
func (a *Volume) ReapableEventTextShort() (*bytes.Buffer, error) {
	return reapableEventText(a, reapableVolumeEventText)
}

// ReapableEventEmail is part of the events.Reapable interface
func (a *Volume) ReapableEventEmail() (owner mail.Address, subject string, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{ErrorText: fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}

	subject = fmt.Sprintf("AWS Resource %s is going to be Reaped!", a.ReapableDescriptionTiny())
	owner = *a.Owner()
	body, err = reapableEventHTML(a, reapableVolumeEventHTMLShort)
	return
}

// ReapableEventEmailShort is part of the events.Reapable interface
func (a *Volume) ReapableEventEmailShort() (owner mail.Address, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{ErrorText: fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}
	owner = *a.Owner()
	body, err = reapableEventHTML(a, reapableVolumeEventHTMLShort)
	return
}

type volumeEventData struct {
	Config                           *Config
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

func (a *Volume) getTemplateData() (interface{}, error) {
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

	return &volumeEventData{
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

// Filter is part of the filter.Filterable interface
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

// AWSConsoleURL returns the url that can be used to access the resource on the AWS Console
func (a *Volume) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/v2/home?region=%s#Volumes:volumeId=%s",
		a.Region.String(), a.Region.String(), url.QueryEscape(a.ID.String())))
	if err != nil {
		log.Error("Error generating AWSConsoleURL. ", err)
	}
	return url
}

// Terminate is a method of reapable.Terminable, which is embedded in reapable.Reapable
func (a *Volume) Terminate() (bool, error) {
	log.Info("Terminating Volume ", a.ReapableDescriptionTiny())
	api := ec2.New(&aws.Config{Region: a.Region.String()})
	idString := a.ID.String()
	input := &ec2.DeleteVolumeInput{
		VolumeID: &idString,
	}
	_, err := api.DeleteVolume(input)
	if err != nil {
		log.Error("could not delete Volume ", a.ReapableDescriptionTiny())
		return false, err
	}
	return true, nil
}

// Stop is a method of reapable.Stoppable, which is embedded in reapable.Reapable
// noop
func (a *Volume) Stop() (bool, error) {
	// use existing min size
	return false, nil
}

// ForceStop is a method of reapable.Stoppable, which is embedded in reapable.Reapable
// noop
func (a *Volume) ForceStop() (bool, error) {
	return false, nil
}
