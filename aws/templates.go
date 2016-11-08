package aws

import (
	"bytes"
	"fmt"
	"net/mail"
	"time"

	htmlTemplate "html/template"
	textTemplate "text/template"

	"github.com/mozilla-services/reaper/reapable"
)

func (a *Resource) getTemplateData() (interface{}, error) {
	ignore1, err := makeIgnoreLink(a.Region(), a.ID(), config.HTTP.TokenSecret, config.HTTP.APIURL, time.Duration(1*24*time.Hour))
	if err != nil {
		return nil, err
	}
	ignore3, err := makeIgnoreLink(a.Region(), a.ID(), config.HTTP.TokenSecret, config.HTTP.APIURL, time.Duration(3*24*time.Hour))
	if err != nil {
		return nil, err
	}
	ignore7, err := makeIgnoreLink(a.Region(), a.ID(), config.HTTP.TokenSecret, config.HTTP.APIURL, time.Duration(7*24*time.Hour))
	if err != nil {
		return nil, err
	}
	terminate, err := makeTerminateLink(a.Region(), a.ID(), config.HTTP.TokenSecret, config.HTTP.APIURL)
	if err != nil {
		return nil, err
	}
	stop, err := makeStopLink(a.Region(), a.ID(), config.HTTP.TokenSecret, config.HTTP.APIURL)
	if err != nil {
		return nil, err
	}
	whitelist, err := makeWhitelistLink(a.Region(), a.ID(), config.HTTP.TokenSecret, config.HTTP.APIURL)
	if err != nil {
		return nil, err
	}

	regionLink := fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/v2/home?region=%s", a.Region(), a.Region())

	return &struct {
		Resource             *Resource
		ResourceType         string
		ResourceName         string
		ID                   string
		Region               string
		AWSConsoleURL        string
		FinalStateTimeString string
		NextStateTimeString  string
		WhitelistTag         string
		RegionLink           string
		TerminateLink        string
		StopLink             string
		WhitelistLink        string
		IgnoreLink1          string
		IgnoreLink3          string
		IgnoreLink7          string
	}{
		Resource:             a,
		ResourceType:         a.ResourceType,
		ResourceName:         a.Name,
		ID:                   a.id.String(),
		Region:               a.region.String(),
		AWSConsoleURL:        a.AWSConsoleURL().String(),
		FinalStateTimeString: a.FinalStateTime().Format(resourceTimeFormat),
		NextStateTimeString:  a.ReaperState().Until.Format(resourceTimeFormat),
		WhitelistTag:         config.WhitelistTag,
		RegionLink:           regionLink,
		TerminateLink:        terminate,
		StopLink:             stop,
		WhitelistLink:        whitelist,
		IgnoreLink1:          ignore1,
		IgnoreLink3:          ignore3,
		IgnoreLink7:          ignore7,
	}, nil
}

// ReapableEventText is part of the events.Reapable interface
func (a *Resource) ReapableEventText() (*bytes.Buffer, error) {
	return reapableEventHTML(a, reapableEventTextTemplate)
}

// ReapableEventTextShort is part of the events.Reapable interface
func (a *Resource) ReapableEventTextShort() (*bytes.Buffer, error) {
	return reapableEventHTML(a, reapableEventTextShortTemplate)
}

// ReapableEventEmail is part of the events.Reapable interface
func (a *Resource) ReapableEventEmail() (owner mail.Address, subject string, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{ErrorText: fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}

	subject = fmt.Sprintf("AWS Resource %s is going to be Reaped!", a.ReapableDescriptionTiny())
	owner = *a.Owner()
	body, err = reapableEventHTML(a, reapableEventHTMLTemplate)
	return
}

// ReapableEventEmailShort is part of the events.Reapable interface
func (a *Resource) ReapableEventEmailShort() (owner mail.Address, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{ErrorText: fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}
	owner = *a.Owner()
	body, err = reapableEventHTML(a, reapableEventHTMLShortTemplate)
	return
}

func reapableEventHTML(a *Resource, text string) (*bytes.Buffer, error) {
	t := htmlTemplate.Must(htmlTemplate.New("reapable").Parse(text))
	buf := bytes.NewBuffer(nil)

	data, err := a.getTemplateData()
	if err != nil {
		return nil, err
	}
	err = t.Execute(buf, data)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func reapableEventText(a *Resource, text string) (*bytes.Buffer, error) {
	t := textTemplate.Must(textTemplate.New("reapable").Parse(text))
	buf := bytes.NewBuffer(nil)

	data, err := a.getTemplateData()
	if err != nil {
		return nil, err
	}
	err = t.Execute(buf, data)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

const reapableEventHTMLTemplate = `
<html>
<body>
	<p>{{.ResourceType}} <a href="{{ .AWSConsoleURL }}">{{ if ne .ResourceName "" }}"{{.ResourceName}}"{{ end }} {{.ID}} in {{.Region}}</a> is scheduled to be terminated.</p>
	<p>You can ignore this message and your {{.ResourceType}} will advance to the next state after <strong>{{ .NextStateTimeString }}</strong>. If you do not take action it will be terminated after <strong>{{ .FinalStateTimeString }}</strong>!</p>
	<p>
		You may also choose to:
		<ul>
			<li><a href="{{ .TerminateLink }}">Terminate it</a></li>
			<li><a href="{{ .StopLink }}">Stop it</a></li>
			<li><a href="{{ .IgnoreLink1 }}">Ignore it for 1 more day</a></li>
			<li><a href="{{ .IgnoreLink3 }}">Ignore it for 3 more days</a></li>
			<li><a href="{{ .IgnoreLink7}}">Ignore it for 7 more days</a></li>
		</ul>
	</p>

	<p>
		If you want the Reaper to ignore this {{.ResourceType}} tag it with {{ .WhitelistTag }} with any value, or click <a href="{{ .WhitelistLink }}">here</a>.
	</p>
</body>
</html>
`

const reapableEventHTMLShortTemplate = `
<html>
<body>
	<p>{{.ResourceType}} <a href="{{ .AWSConsoleURL }}">{{ if ne .ResourceName "" }}"{{.ResourceName}}" {{ end }}</a> in {{.Region}}</a> will advance to the next state after <strong>{{ .NextStateTimeString }}</strong> and be terminated after <strong>{{ .FinalStateTimeString }}</strong>!</p>
		<br />
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

const reapableEventTextShortTemplate = `%%%
{{.ResourceType}} [{{.ID}}]({{.AWSConsoleURL}}) in region: [{{.Region}}]({{.RegionLink}}).\n
{{if .Resource.Owned}} Owned by {{.Resource.Owner}}.\n{{end}}
This Resource is scheduled to be terminated after <strong>{{ .FinalStateTimeString }}</strong>\n
[Whitelist]({{ .WhitelistLink }}), [Ignore it for 1 more day]({{ .IgnoreLink1 }}), [3 days]{{ .IgnoreLink3 }}, [7 days]{{ .IgnoreLink7}}, [Stop]({{ .StopLink }}), or [Terminate]({{ .TerminateLink }}) this Resource.
%%%`

const reapableEventTextTemplate = `%%%
Reaper has discovered an Resource qualified as reapable: [{{.ID}}]({{.AWSConsoleURL}}) in region: [{{.Region}}](https://{{.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.Region}}).\n
{{if .Resource.Owned}}Owned by {{.Resource.Owner}}.\n{{end}}
{{ if .AWSConsoleURL}}{{.AWSConsoleURL}}\n{{end}}
[AWS Console URL]({{.AWSConsoleURL}})\n
This Resource is scheduled to be terminated after <strong>{{ .FinalStateTimeString }}</strong>\n
[Whitelist]({{ .WhitelistLink }}) this Resource.
[Ignore it for 1 more day]({{ .IgnoreLink1 }})
[3 days]{{ .IgnoreLink3 }}
[7 days]{{ .IgnoreLink7}}
[Stop]({{ .StopLink }}) this Resource.
[Terminate]({{ .TerminateLink }}) this Resource.
%%%`
