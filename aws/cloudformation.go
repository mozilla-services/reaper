package aws

import (
	"bytes"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"

	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

// Cloudformation is a Reapable, Filterable
// embeds AWS API's cloudformation.Stack
type Cloudformation struct {
	Resource
	cloudformation.Stack
	Resources []cloudformation.StackResource
	// locks because of CloudformationResources access
	sync.RWMutex
}

// NewCloudformation creates a new Cloudformation from the AWS API's cloudformation.Stack
func NewCloudformation(region string, stack *cloudformation.Stack) *Cloudformation {
	a := Cloudformation{
		Resource: Resource{
			Region:      reapable.Region(region),
			ID:          reapable.ID(*stack.StackID),
			Name:        *stack.StackName,
			Tags:        make(map[string]string),
			reaperState: state.NewStateWithUntil(time.Now().Add(config.Notifications.FirstStateDuration.Duration)),
		},
		Stack: *stack,
	}

	a.Lock()
	defer a.Unlock()

	// because getting resources is rate limited...
	go func() {
		a.Lock()
		defer a.Unlock()
		for resource := range cloudformationResources(a.Region.String(), a.ID.String()) {
			a.Resources = append(a.Resources, *resource)
		}
	}()

	for _, tag := range stack.Tags {
		a.Resource.Tags[*tag.Key] = *tag.Value
	}

	if a.Tagged(reaperTag) {
		// restore previously tagged state
		a.reaperState = state.NewStateWithTag(a.Resource.Tag(reaperTag))
	} else {
		// initial state
		a.reaperState = state.NewState()
	}

	return &a
}

// ReapableEventText is part of the events.Reapable interface
func (a *Cloudformation) ReapableEventText() (*bytes.Buffer, error) {
	return reapableEventText(a, reapableCloudformationEventText)
}

// ReapableEventTextShort is part of the events.Reapable interface
func (a *Cloudformation) ReapableEventTextShort() (*bytes.Buffer, error) {
	return reapableEventText(a, reapableCloudformationEventTextShort)
}

// ReapableEventEmail is part of the events.Reapable interface
func (a *Cloudformation) ReapableEventEmail() (owner mail.Address, subject string, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}

	subject = fmt.Sprintf("AWS Resource %s is going to be Reaped!", a.ReapableDescriptionTiny())
	owner = *a.Owner()
	body, err = reapableEventHTML(a, reapableCloudformationEventHTML)
	return
}

// ReapableEventEmailShort is part of the events.Reapable interface
func (a *Cloudformation) ReapableEventEmailShort() (owner mail.Address, body *bytes.Buffer, err error) {
	// if unowned, return unowned error
	if !a.Owned() {
		err = reapable.UnownedError{fmt.Sprintf("%s does not have an owner tag", a.ReapableDescriptionShort())}
		return
	}
	owner = *a.Owner()
	body, err = reapableEventHTML(a, reapableCloudformationEventHTMLShort)
	return
}

type cloudformationEventData struct {
	Config         *Config
	Cloudformation *Cloudformation
	TerminateLink  string
	StopLink       string
	ForceStopLink  string
	WhitelistLink  string
	IgnoreLink1    string
	IgnoreLink3    string
	IgnoreLink7    string
}

func (a *Cloudformation) getTemplateData() (interface{}, error) {
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

	return &cloudformationEventData{
		Config:         config,
		Cloudformation: a,
		TerminateLink:  terminate,
		StopLink:       stop,
		ForceStopLink:  forcestop,
		WhitelistLink:  whitelist,
		IgnoreLink1:    ignore1,
		IgnoreLink3:    ignore3,
		IgnoreLink7:    ignore7,
	}, nil
}

const reapableCloudformationEventHTML = `
<html>
<body>
	<p>Cloudformation <a href="{{ .Cloudformation.AWSConsoleURL }}">{{ if .Cloudformation.Name }}"{{.Cloudformation.Name}}" {{ end }} in {{.Cloudformation.Region}}</a> is scheduled to be terminated.</p>

	<p>
		You can ignore this message and your Cloudformation will advance to the next state after <strong>{{.Cloudformation.ReaperState.Until.UTC.Format "Jan 2, 2006 at 3:04pm (MST)"}}</strong>. If you do not take action it will be terminated!
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
		If you want the Reaper to ignore this Cloudformation tag it with {{ .Config.WhitelistTag }} with any value, or click <a href="{{ .WhitelistLink }}">here</a>.
	</p>
</body>
</html>
`

const reapableCloudformationEventHTMLShort = `
<html>
<body>
	<p>Cloudformation <a href="{{ .Cloudformation.AWSConsoleURL }}">{{ if .Cloudformation.Name }}"{{.Cloudformation.Name}}" {{ end }}</a> in {{.Cloudformation.Region}}</a> is scheduled to be terminated after <strong>{{.Cloudformation.ReaperState.Until.UTC.Format "Jan 2, 2006 at 3:04pm (MST)"}}</strong>.
		<br />
		<a href="{{ .TerminateLink }}">Terminate</a>,
		<a href="{{ .IgnoreLink1 }}">Ignore it for 1 more day</a>,
		<a href="{{ .IgnoreLink3 }}">3 days</a>,
		<a href="{{ .IgnoreLink7}}"> 7 days</a>, or
		<a href="{{ .WhitelistLink }}">Whitelist</a> it.
	</p>
</body>
</html>
`

const reapableCloudformationEventTextShort = `%%%
Cloudformation [{{.Cloudformation.ID}}]({{.Cloudformation.AWSConsoleURL}}) in region: [{{.Cloudformation.Region}}](https://{{.Cloudformation.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.Cloudformation.Region}}).{{if .Cloudformation.Owned}} Owned by {{.Cloudformation.Owner}}.\n{{end}}
[Whitelist]({{ .WhitelistLink }}), or [Terminate]({{ .TerminateLink }}) this Cloudformation.
%%%`

const reapableCloudformationEventText = `%%%
Reaper has discovered a Cloudformation qualified as reapable: [{{.Cloudformation.ID}}]({{.Cloudformation.AWSConsoleURL}}) in region: [{{.Cloudformation.Region}}](https://{{.Cloudformation.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.Cloudformation.Region}}).\n
{{if .Cloudformation.Owned}}Owned by {{.Cloudformation.Owner}}.\n{{end}}
{{ if .Cloudformation.AWSConsoleURL}}{{.Cloudformation.AWSConsoleURL}}\n{{end}}
[AWS Console URL]({{.Cloudformation.AWSConsoleURL}})\n
[Whitelist]({{ .WhitelistLink }}) this Cloudformation.
[Terminate]({{ .TerminateLink }}) this Cloudformation.
%%%`

// Save is part of reapable.Saveable, which embedded in reapable.Reapable
// no op because we cannot tag cloudformations without updating the stack
func (a *Cloudformation) Save(s *state.State) (bool, error) {
	return false, nil
}

// Unsave is part of reapable.Saveable, which embedded in reapable.Reapable
// no op because we cannot tag cloudformations without updating the stack
func (a *Cloudformation) Unsave() (bool, error) {
	log.Info("Unsaving %s", a.ReapableDescriptionTiny())
	return false, nil
}

// Filter is part of the filter.Filterable interface
func (a *Cloudformation) Filter(filter filters.Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
	case "Status":
		if a.StackStatus != nil && *a.StackStatus == filter.Arguments[0] {
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
		if a.StackStatus != nil && *a.StackStatus != filter.Arguments[0] {
			matched = true
		}
	case "CreatedTimeInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && a.CreationTime != nil && time.Since(*a.CreationTime) < d {
			matched = true
		}
	case "CreatedTimeNotInTheLast":
		d, err := time.ParseDuration(filter.Arguments[0])
		if err == nil && a.CreationTime != nil && time.Since(*a.CreationTime) > d {
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
		log.Error("No function %s could be found for filtering Cloudformations.", filter.Function)
	}
	return matched
}

// AWSConsoleURL returns the url that can be used to access the resource on the AWS Console
func (a *Cloudformation) AWSConsoleURL() *url.URL {
	url, err := url.Parse("https://console.aws.amazon.com/cloudformation/home")
	// setting RawQuery because QueryEscape messes with the "/"s in the url
	url.RawQuery = fmt.Sprintf("region=%s#/stacks?filter=active&tab=overview&stackId=%s", a.Region.String(), a.ID.String())
	if err != nil {
		log.Error("Error generating AWSConsoleURL. %s", err)
	}
	return url
}

// Terminate is a method of reapable.Terminable, which is embedded in reapable.Reapable
func (a *Cloudformation) Terminate() (bool, error) {
	log.Info("Terminating Cloudformation %s", a.ReapableDescriptionTiny())
	as := cloudformation.New(&aws.Config{Region: string(a.Region)})

	stringID := string(a.ID)

	input := &cloudformation.DeleteStackInput{
		StackName: &stringID,
	}
	_, err := as.DeleteStack(input)
	if err != nil {
		log.Error("could not delete Cloudformation %s", a.ReapableDescriptionTiny())
		return false, err
	}
	return false, nil
}

// Whitelist is a method of reapable.Whitelistable, which is embedded in reapable.Reapable
// no op because we cannot tag cloudformations without updating the stack
func (a *Cloudformation) Whitelist() (bool, error) {
	return false, nil
}

// Stop is a method of reapable.Stoppable, which is embedded in reapable.Reapable
// no op because there is no concept of stopping a cloudformation
func (a *Cloudformation) Stop() (bool, error) {
	return false, nil
}

// ForceStop is a method of reapable.Stoppable, which is embedded in reapable.Reapable
// no op because there is no concept of stopping a cloudformation
func (a *Cloudformation) ForceStop() (bool, error) {
	return false, nil
}
