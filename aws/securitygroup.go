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
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

type SecurityGroup struct {
	AWSResource
	ec2.SecurityGroup
}

func NewSecurityGroup(region string, sg *ec2.SecurityGroup) *SecurityGroup {
	if sg == nil {
		return nil
	}
	s := SecurityGroup{
		AWSResource: AWSResource{
			ID:     reapable.ID(*sg.GroupID),
			Name:   *sg.GroupName,
			Region: reapable.Region(region),
			Tags:   make(map[string]string),
		},
		SecurityGroup: *sg,
	}

	for _, tag := range sg.Tags {
		s.AWSResource.Tags[*tag.Key] = *tag.Value
	}
	if s.Tagged(reaperTag) {
		// restore previously tagged state
		s.reaperState = state.NewStateWithTag(s.AWSResource.Tag(reaperTag))
	} else {
		// initial state
		s.reaperState = state.NewStateWithUntilAndState(
			time.Now().Add(config.Notifications.FirstStateDuration.Duration),
			state.FirstState)
	}

	return &s
}

func (a *SecurityGroup) reapableEventHTML(text string) *bytes.Buffer {
	t := htmlTemplate.Must(htmlTemplate.New("reapable").Parse(text))
	buf := bytes.NewBuffer(nil)

	data, err := a.getTemplateData()
	err = t.Execute(buf, data)
	if err != nil {
		log.Debug(fmt.Sprintf("Template generation error: %s", err))
	}
	return buf
}

func (a *SecurityGroup) reapableEventText(text string) *bytes.Buffer {
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

func (a *SecurityGroup) ReapableEventText() *bytes.Buffer {
	return a.reapableEventText(reapableSecurityGroupEventText)
}

func (a *SecurityGroup) ReapableEventTextShort() *bytes.Buffer {
	return a.reapableEventText(reapableSecurityGroupEventTextShort)
}

func (a *SecurityGroup) ReapableEventEmail() (owner mail.Address, subject string, body string, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}

	subject = fmt.Sprintf("AWS Resource %s is going to be Reaped!", a.ReapableDescriptionTiny())
	owner = *a.Owner()
	body = a.reapableEventHTML(reapableSecurityGroupEventHTML).String()
	return
}

func (a *SecurityGroup) ReapableEventEmailShort() (owner mail.Address, body string, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}
	owner = *a.Owner()
	body = a.reapableEventHTML(reapableSecurityGroupEventHTMLShort).String()
	return
}

type SecurityGroupEventData struct {
	Config        *AWSConfig
	SecurityGroup *SecurityGroup
	TerminateLink string
	StopLink      string
	ForceStopLink string
	WhitelistLink string
	IgnoreLink1   string
	IgnoreLink3   string
	IgnoreLink7   string
}

func (a *SecurityGroup) getTemplateData() (*SecurityGroupEventData, error) {
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

	return &SecurityGroupEventData{
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
	<p>SecurityGroup <a href="{{ .SecurityGroup.AWSConsoleURL }}">{{ if .SecurityGroup.Name }}"{{.SecurityGroup.Name}}" {{ end }} in {{.SecurityGroup.Region}}</a> is scheduled to be terminated.</p>

	<p>
		You can ignore this message and your SecurityGroup will advance to the next state after <strong>{{.SecurityGroup.ReaperState.Until}}</strong>. If you do not take action it will be terminated!
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
		If you want the Reaper to ignore this SecurityGroup tag it with {{ .Config.WhitelistTag }} with any value, or click <a href="{{ .WhitelistLink }}">here</a>.
	</p>
</body>
</html>
`

const reapableSecurityGroupEventHTMLShort = `
<html>
<body>
	<p>SecurityGroup <a href="{{ .SecurityGroup.AWSConsoleURL }}">{{ if .SecurityGroup.Name }}"{{.SecurityGroup.Name}}" {{ end }}</a> in {{.SecurityGroup.Region}}</a> is scheduled to be terminated after <strong>{{.SecurityGroup.ReaperState.Until}}</strong>.
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

const reapableSecurityGroupEventTextShort = `%%%
SecurityGroup [{{.SecurityGroup.ID}}]({{.SecurityGroup.AWSConsoleURL}}) in region: [{{.SecurityGroup.Region}}](https://{{.SecurityGroup.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.SecurityGroup.Region}}).{{if .SecurityGroup.Owned}} Owned by {{.SecurityGroup.Owner}}.\n{{end}}
[Whitelist]({{ .WhitelistLink }}), [Scale to 0]({{ .StopLink }}), [ForceScale to 0]({{ .ForceStopLink }}), or [Terminate]({{ .TerminateLink }}) this SecurityGroup.
%%%`

const reapableSecurityGroupEventText = `%%%
Reaper has discovered an SecurityGroup qualified as reapable: [{{.SecurityGroup.ID}}]({{.SecurityGroup.AWSConsoleURL}}) in region: [{{.SecurityGroup.Region}}](https://{{.SecurityGroup.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.SecurityGroup.Region}}).\n
{{if .SecurityGroup.Owned}}Owned by {{.SecurityGroup.Owner}}.\n{{end}}
{{ if .SecurityGroup.AWSConsoleURL}}{{.SecurityGroup.AWSConsoleURL}}\n{{end}}
[AWS Console URL]({{.SecurityGroup.AWSConsoleURL}})\n
[Whitelist]({{ .WhitelistLink }}) this SecurityGroup.
[Terminate]({{ .TerminateLink }}) this SecurityGroup.
%%%`

func (a *SecurityGroup) Filter(filter filters.Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
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
	case "Named":
		if a.Name == filter.Arguments[0] {
			matched = true
		}
	case "NotNamed":
		if a.Name != filter.Arguments[0] {
			matched = true
		}
	case "InCloudformation":
		return a.IsInCloudformation
	case "NotInCloudformation":
		return !a.IsInCloudformation
	default:
		log.Error(fmt.Sprintf("No function %s could be found for filtering SecurityGroups.", filter.Function))
	}
	return matched
}

func (a *SecurityGroup) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/ec2/home?region=%s#SecurityGroups:id=%s;view=details",
		string(a.Region), string(a.Region), url.QueryEscape(string(a.ID))))
	if err != nil {
		log.Error(fmt.Sprintf("Error generating AWSConsoleURL. %s", err))
	}
	return url
}

func (a *SecurityGroup) Terminate() (bool, error) {
	log.Notice("Terminating SecurityGroup %s", a.ReapableDescriptionTiny())
	as := ec2.New(&aws.Config{Region: string(a.Region)})

	// ugh this is stupid
	stringID := string(a.ID)

	input := &ec2.DeleteSecurityGroupInput{
		GroupName: &stringID,
	}
	_, err := as.DeleteSecurityGroup(input)
	if err != nil {
		log.Error(fmt.Sprintf("could not delete SecurityGroup %s", a.ReapableDescriptionTiny()))
		return false, err
	}
	return false, nil
}

// noop
func (a *SecurityGroup) Stop() (bool, error) {
	return false, nil
}

// noop
func (a *SecurityGroup) ForceStop() (bool, error) {
	return false, nil
}
