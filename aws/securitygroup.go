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

// SecurityGroup is a Reapable, Filterable
// embeds AWS API's ec2.SecurityGroup
type SecurityGroup struct {
	Resource
	ec2.SecurityGroup
}

// NewSecurityGroup creates an SecurityGroup from the AWS API's ec2.SecurityGroup
func NewSecurityGroup(region string, sg *ec2.SecurityGroup) *SecurityGroup {
	s := SecurityGroup{
		Resource: Resource{
			ID:     reapable.ID(*sg.GroupID),
			Name:   *sg.GroupName,
			Region: reapable.Region(region),
			Tags:   make(map[string]string),
		},
		SecurityGroup: *sg,
	}

	for _, tag := range sg.Tags {
		s.Resource.Tags[*tag.Key] = *tag.Value
	}
	if s.Tagged(reaperTag) {
		// restore previously tagged state
		s.reaperState = state.NewStateWithTag(s.Resource.Tag(reaperTag))
	} else {
		// initial state
		s.reaperState = state.NewState()
	}

	return &s
}

// ReapableEventText is part of the events.Reapable interface
func (a *SecurityGroup) ReapableEventText() (*bytes.Buffer, error) {
	return reapableEventText(a, reapableSecurityGroupEventText)
}

// ReapableEventTextShort is part of the events.Reapable interface
func (a *SecurityGroup) ReapableEventTextShort() (*bytes.Buffer, error) {
	return reapableEventText(a, reapableSecurityGroupEventTextShort)
}

// ReapableEventEmail is part of the events.Reapable interface
func (a *SecurityGroup) ReapableEventEmail() (owner mail.Address, subject string, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{ErrorText: fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}

	subject = fmt.Sprintf("AWS Resource %s is going to be Reaped!", a.ReapableDescriptionTiny())
	owner = *a.Owner()
	body, err = reapableEventHTML(a, reapableSecurityGroupEventHTML)
	return
}

// ReapableEventEmailShort is part of the events.Reapable interface
func (a *SecurityGroup) ReapableEventEmailShort() (owner mail.Address, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{ErrorText: fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}
	owner = *a.Owner()
	body, err = reapableEventHTML(a, reapableSecurityGroupEventHTMLShort)
	return
}

type securityGroupEventData struct {
	Config        *Config
	SecurityGroup *SecurityGroup
	TerminateLink string
	StopLink      string
	ForceStopLink string
	WhitelistLink string
	IgnoreLink1   string
	IgnoreLink3   string
	IgnoreLink7   string
}

func (a *SecurityGroup) getTemplateData() (interface{}, error) {
	ignore1, err := makeIgnoreLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.APIURL, time.Duration(1*24*time.Hour))
	ignore3, err := makeIgnoreLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.APIURL, time.Duration(3*24*time.Hour))
	ignore7, err := makeIgnoreLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.APIURL, time.Duration(7*24*time.Hour))
	terminate, err := makeTerminateLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.APIURL)
	stop, err := makeStopLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.APIURL)
	forcestop, err := makeForceStopLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.APIURL)
	whitelist, err := makeWhitelistLink(a.Region, a.ID, config.HTTP.TokenSecret, config.HTTP.APIURL)
	if err != nil {
		return nil, err
	}

	return &securityGroupEventData{
		Config:        config,
		SecurityGroup: a,
		TerminateLink: terminate,
		StopLink:      stop,
		ForceStopLink: forcestop,
		WhitelistLink: whitelist,
		IgnoreLink1:   ignore1,
		IgnoreLink3:   ignore3,
		IgnoreLink7:   ignore7,
	}, nil
}

const reapableSecurityGroupEventHTML = `
<html>
<body>
	<p>SecurityGroup <a href="{{ .SecurityGroup.AWSConsoleURL }}">{{ if .SecurityGroup.Name }}"{{.SecurityGroup.Name}}" {{ end }} in {{.SecurityGroup.Region}}</a> is scheduled to be deleted.</p>

	<p>
		You can ignore this message and your SecurityGroup will advance to the next state after <strong>{{.SecurityGroup.ReaperState.Until}}</strong>. If you do not take action it will be deleted!
	</p>

	<p>
		You may also choose to:
		<ul>
			<li><a href="{{ .TerminateLink }}">Delete it now</a></li>
			<li><a href="{{ .IgnoreLink1 }}">Ignore it for 1 more day</a></li>
			<li><a href="{{ .IgnoreLink3 }}">Ignore it for 3 more days</a></li>
			<li><a href="{{ .IgnoreLink7}}">Ignore it for 7 more days</a></li>
			<li><a href="{{ .WhitelistLink }}">Whitelist</a> it.</li>
		</ul>
	</p>

	<p>
		If you want the Reaper to ignore this SecurityGroup tag it with {{ .Config.WhitelistTag }} with any value, or click <a href="{{ .WhitelistLink }}">here</a>.
	</p>
</body>
</html>
`

const reapableSecurityGroupEventHTMLShort = `
<html>
<body>
	<p>SecurityGroup <a href="{{ .SecurityGroup.AWSConsoleURL }}">{{ if .SecurityGroup.Name }}"{{.SecurityGroup.Name}}" {{ end }}</a> in {{.SecurityGroup.Region}}</a> is scheduled to be deleted after <strong>{{.SecurityGroup.ReaperState.Until}}</strong>.
		<br />
		<a href="{{ .TerminateLink }}">Delete</a>,
		<a href="{{ .IgnoreLink1 }}">Ignore it for 1 more day</a>,
		<a href="{{ .IgnoreLink3 }}">3 days</a>,
		<a href="{{ .IgnoreLink7}}"> 7 days</a>, or
		<a href="{{ .WhitelistLink }}">Whitelist</a> it.
	</p>
</body>
</html>
`

const reapableSecurityGroupEventTextShort = `%%%
SecurityGroup [{{.SecurityGroup.ID}}]({{.SecurityGroup.AWSConsoleURL}}) in region: [{{.SecurityGroup.Region}}](https://{{.SecurityGroup.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.SecurityGroup.Region}}).{{if .SecurityGroup.Owned}} Owned by {{.SecurityGroup.Owner}}.\n{{end}}
[Whitelist]({{ .WhitelistLink }}) or [Delete]({{ .TerminateLink }}) this SecurityGroup.
%%%`

const reapableSecurityGroupEventText = `%%%
Reaper has discovered an SecurityGroup qualified as reapable: [{{.SecurityGroup.ID}}]({{.SecurityGroup.AWSConsoleURL}}) in region: [{{.SecurityGroup.Region}}](https://{{.SecurityGroup.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.SecurityGroup.Region}}).\n
{{if .SecurityGroup.Owned}}Owned by {{.SecurityGroup.Owner}}.\n{{end}}
{{ if .SecurityGroup.AWSConsoleURL}}{{.SecurityGroup.AWSConsoleURL}}\n{{end}}
[AWS Console URL]({{.SecurityGroup.AWSConsoleURL}})\n
[Whitelist]({{ .WhitelistLink }}) this SecurityGroup.
[Delete]({{ .TerminateLink }}) this SecurityGroup.
%%%`

// Filter is part of the filter.Filterable interface
func (a *SecurityGroup) Filter(filter filters.Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
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
		log.Error(fmt.Sprintf("No function %s could be found for filtering SecurityGroups.", filter.Function))
	}
	return matched
}

// AWSConsoleURL returns the url that can be used to access the resource on the AWS Console
func (a *SecurityGroup) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/v2/home?region=%s#SecurityGroups:id=%s;view=details",
		string(a.Region), string(a.Region), url.QueryEscape(string(a.ID))))
	if err != nil {
		log.Error("Error generating AWSConsoleURL. ", err)
	}
	return url
}

// Terminate is a method of reapable.Terminable, which is embedded in reapable.Reapable
func (a *SecurityGroup) Terminate() (bool, error) {
	log.Info("Terminating SecurityGroup ", a.ReapableDescriptionTiny())
	as := ec2.New(&aws.Config{Region: string(a.Region)})

	// ugh this is stupid
	stringID := string(a.ID)

	input := &ec2.DeleteSecurityGroupInput{
		GroupName: &stringID,
	}
	_, err := as.DeleteSecurityGroup(input)
	if err != nil {
		log.Error("could not delete SecurityGroup ", a.ReapableDescriptionTiny())
		return false, err
	}
	return false, nil
}

// Stop is a method of reapable.Stoppable, which is embedded in reapable.Reapable
// noop
func (a *SecurityGroup) Stop() (bool, error) {
	return false, nil
}

// ForceStop is a method of reapable.Stoppable, which is embedded in reapable.Reapable
// noop
func (a *SecurityGroup) ForceStop() (bool, error) {
	return false, nil
}
