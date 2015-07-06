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
	"github.com/aws/aws-sdk-go/service/cloudformation"

	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

type CloudformationStack struct {
	AWSResource
	cloudformation.Stack
}

func NewCloudformationStack(region string, stack *cloudformation.Stack) *CloudformationStack {
	a := CloudformationStack{
		AWSResource: AWSResource{
			Region:      reapable.Region(region),
			ID:          reapable.ID(*stack.StackID),
			Name:        *stack.StackName,
			Tags:        make(map[string]string),
			reaperState: state.NewStateWithUntil(time.Now().Add(config.Notifications.FirstStateDuration.Duration)),
		},
		Stack: *stack,
	}

	for i := 0; i < len(stack.Tags); i++ {
		a.AWSResource.Tags[*stack.Tags[i].Key] = *stack.Tags[i].Value
	}

	if a.Tagged(reaperTag) {
		// restore previously tagged state
		a.reaperState = state.NewStateWithTag(a.AWSResource.Tags[reaperTag])
	} else {
		// initial state
		a.reaperState = state.NewStateWithUntilAndState(
			time.Now().Add(config.Notifications.FirstStateDuration.Duration),
			state.FirstState)
	}

	return &a
}

func (a *CloudformationStack) reapableEventHTML(text string) *bytes.Buffer {
	t := htmlTemplate.Must(htmlTemplate.New("reapable").Parse(text))
	buf := bytes.NewBuffer(nil)

	data, err := a.getTemplateData()
	err = t.Execute(buf, data)
	if err != nil {
		log.Debug(fmt.Sprintf("Template generation error: %s", err))
	}
	return buf
}

func (a *CloudformationStack) reapableEventText(text string) *bytes.Buffer {
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

func (a *CloudformationStack) ReapableEventText() *bytes.Buffer {
	return a.reapableEventText(reapableCloudformationEventText)
}

func (a *CloudformationStack) ReapableEventTextShort() *bytes.Buffer {
	return a.reapableEventText(reapableCloudformationEventTextShort)
}

func (a *CloudformationStack) ReapableEventEmail() (owner mail.Address, subject string, body string, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}

	subject = fmt.Sprintf("AWS Resource %s is going to be Reaped!", a.ReapableDescriptionTiny())
	owner = *a.Owner()
	body = a.reapableEventHTML(reapableCloudformationEventHTML).String()
	return
}

func (a *CloudformationStack) ReapableEventEmailShort() (owner mail.Address, body string, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}
	owner = *a.Owner()
	body = a.reapableEventHTML(reapableCloudformationEventHTMLShort).String()
	return
}

type CloudformationStackEventData struct {
	Config              *AWSConfig
	CloudformationStack *CloudformationStack
	TerminateLink       string
	StopLink            string
	ForceStopLink       string
	WhitelistLink       string
	IgnoreLink1         string
	IgnoreLink3         string
	IgnoreLink7         string
}

func (a *CloudformationStack) getTemplateData() (*CloudformationStackEventData, error) {
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

	return &CloudformationStackEventData{
		Config:              config,
		CloudformationStack: a,
		TerminateLink:       terminate,
		StopLink:            stop,
		ForceStopLink:       forcestop,
		WhitelistLink:       whitelist,
		IgnoreLink1:         ignore1,
		IgnoreLink3:         ignore3,
		IgnoreLink7:         ignore7,
	}, nil
}

const reapableCloudformationEventHTML = `
<html>
<body>
	<p>CloudformationStack <a href="{{ .CloudformationStack.AWSConsoleURL }}">{{ if .CloudformationStack.Name }}"{{.CloudformationStack.Name}}" {{ end }} in {{.CloudformationStack.Region}}</a> is scheduled to be terminated.</p>

	<p>
		You can ignore this message and your CloudformationStack will advance to the next state after <strong>{{.CloudformationStack.ReaperState.Until}}</strong>. If you do not take action it will be terminated!
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
		If you want the Reaper to ignore this CloudformationStack tag it with {{ .Config.WhitelistTag }} with any value, or click <a href="{{ .WhitelistLink }}">here</a>.
	</p>
</body>
</html>
`

const reapableCloudformationEventHTMLShort = `
<html>
<body>
	<p>CloudformationStack <a href="{{ .CloudformationStack.AWSConsoleURL }}">{{ if .CloudformationStack.Name }}"{{.CloudformationStack.Name}}" {{ end }}</a> in {{.CloudformationStack.Region}}</a> is scheduled to be terminated after <strong>{{.CloudformationStack.ReaperState.Until}}</strong>.
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

const reapableCloudformationEventTextShort = `%%%
CloudformationStack [{{.CloudformationStack.ID}}]({{.CloudformationStack.AWSConsoleURL}}) in region: [{{.CloudformationStack.Region}}](https://{{.CloudformationStack.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.CloudformationStack.Region}}).{{if .CloudformationStack.Owned}} Owned by {{.CloudformationStack.Owner}}.\n{{end}}
[Whitelist]({{ .WhitelistLink }}), or [Terminate]({{ .TerminateLink }}) this CloudformationStack.
%%%`

const reapableCloudformationEventText = `%%%
Reaper has discovered an CloudformationStack qualified as reapable: [{{.CloudformationStack.ID}}]({{.CloudformationStack.AWSConsoleURL}}) in region: [{{.CloudformationStack.Region}}](https://{{.CloudformationStack.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.CloudformationStack.Region}}).\n
{{if .CloudformationStack.Owned}}Owned by {{.CloudformationStack.Owner}}.\n{{end}}
{{ if .CloudformationStack.AWSConsoleURL}}{{.CloudformationStack.AWSConsoleURL}}\n{{end}}
[AWS Console URL]({{.CloudformationStack.AWSConsoleURL}})\n
[Whitelist]({{ .WhitelistLink }}) this CloudformationStack.
[Terminate]({{ .TerminateLink }}) this CloudformationStack.
%%%`

// method for reapable -> overrides promoted AWSResource method of same name?
func (a *CloudformationStack) Save(s *state.State) (bool, error) {
	return a.tagReaperState(a.Region, a.ID, a.ReaperState())
}

// method for reapable -> overrides promoted AWSResource method of same name?
func (a *CloudformationStack) Unsave() (bool, error) {
	log.Notice("Unsaving %s", a.ReapableDescriptionTiny())
	return a.untagReaperState(a.Region, a.ID, a.ReaperState())
}

func (a *CloudformationStack) untagReaperState(region reapable.Region, id reapable.ID, newState *state.State) (bool, error) {
	return false, nil
}

func (a *CloudformationStack) tagReaperState(region reapable.Region, id reapable.ID, newState *state.State) (bool, error) {
	return false, nil
}

func (a *CloudformationStack) Filter(filter filters.Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
	case "Status":
		if *a.StackStatus == filter.Arguments[0] {
			// one of:
			// CREATE_COMPLETE
			// CREATE_IN_PROGRESS
			// CREATE_FAILED
			// DELETE_COMPLETE
			// DELETE_FAILED
			// DELETE_IN_PROGRESS
			// ROLLBACK_COMPLETE
			// ROLLBACK_FAILED
			// ROLLBACK_IN_PROGRESS
			// UPDATE_COMPLETE
			// UPDATE_COMPLETE_CLEANUP_IN_PROGRESS
			// UPDATE_IN_PROGRESS
			// UPDATE_ROLLBACK_COMPLETE
			// UPDATE_ROLLBACK_COMPLETE_CLEANUP_IN_PROGRESS
			// UPDATE_ROLLBACK_FAILED
			// UPDATE_ROLLBACK_IN_PROGRESS
			matched = true
		}
	case "NotStatus":
		if *a.StackStatus != filter.Arguments[0] {
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
	case "CreatedTimeInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && time.Since(*a.CreationTime) < d {
			matched = true
		}
	case "CreatedTimeNotInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && time.Since(*a.CreationTime) > d {
			matched = true
		}
	default:
		log.Error(fmt.Sprintf("No function %s could be found for filtering CloudformationStacks.", filter.Function))
	}
	return matched
}

func (a *CloudformationStack) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/cloudformation/home?region=%s#/stacks?filter=active&tab=overview&stackId=%s",
		string(a.Region), string(a.Region), url.QueryEscape(string(a.ID))))
	if err != nil {
		log.Error(fmt.Sprintf("Error generating AWSConsoleURL. %s", err))
	}
	return url
}

func (a *CloudformationStack) Terminate() (bool, error) {
	log.Notice("Terminating CloudformationStack %s", a.ReapableDescriptionTiny())
	as := cloudformation.New(&aws.Config{Region: string(a.Region)})

	stringID := string(a.ID)

	input := &cloudformation.DeleteStackInput{
		StackName: &stringID,
	}
	_, err := as.DeleteStack(input)
	if err != nil {
		log.Error(fmt.Sprintf("could not delete CloudformationStack %s", a.ReapableDescriptionTiny()))
		return false, err
	}
	return false, nil
}

func (a *CloudformationStack) Stop() (bool, error) {
	return false, nil
}

func (a *CloudformationStack) ForceStop() (bool, error) {
	return false, nil
}
