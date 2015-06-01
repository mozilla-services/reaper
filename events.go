package main

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/PagerDuty/godspeed"
)

type EventReporter interface {
	NewEvent(title string, text string, fields map[string]string, tags []string)
	NewStatistic(name string, value float64, tags []string)
	NewReapableInstanceEvent(i *Instance)
	NewReapableASGEvent(a *AutoScalingGroup)
}

// implements EventReporter but does nothing
type NoEventReporter struct{}

func (n NoEventReporter) NewEvent(title string, text string, fields map[string]string, tags []string) {
}
func (n NoEventReporter) NewStatistic(name string, value float64, tags []string) {}
func (n NoEventReporter) NewReapableInstanceEvent(i *Instance)                   {}
func (n NoEventReporter) NewReapableASGEvent(a *AutoScalingGroup)                {}

// implements EventReporter, sends events and statistics to DataDog
// uses godspeed, requires dd-agent running
type DataDog struct {
	eventTemplate template.Template
}

// TODO: make this async?
// TODO: not recreate godspeed
func (d DataDog) NewEvent(title string, text string, fields map[string]string, tags []string) {
	g, err := godspeed.NewDefault()
	if err != nil {
		Log.Debug("Error creating Godspeed, ", err)
	}
	defer g.Conn.Close()
	err = g.Event(title, text, fields, tags)
	if err != nil {
		Log.Debug(fmt.Sprintf("Error reporting Godspeed event %s", title), err)
	}
	Log.Debug(fmt.Sprintf("Event %s posted to DataDog.", title))
}

func (d DataDog) NewStatistic(name string, value float64, tags []string) {
	g, err := godspeed.NewDefault()
	if err != nil {
		Log.Debug("Error creating Godspeed, ", err)
	}
	defer g.Conn.Close()
	err = g.Gauge(name, value, tags)
	if err != nil {
		Log.Debug(fmt.Sprintf("Error reporting Godspeed statistic %s", name), err)
	}

	Log.Debug(fmt.Sprintf("Statistic %s posted to DataDog.", name))
}

type InstanceEventData struct {
	Config   *Config
	Instance *Instance
}

type ASGEventData struct {
	Config *Config
	ASG    *AutoScalingGroup
}

func (d DataDog) NewReapableASGEvent(a *AutoScalingGroup) {
	var funcMap = template.FuncMap{
		"MakeTerminateLink": MakeTerminateLink,
		"MakeIgnoreLink":    MakeIgnoreLink,
		"MakeWhitelistLink": MakeWhitelistLink,
	}
	t := template.Must(template.New("reapable").Funcs(funcMap).Parse(reapableASGTemplateDataDog))
	buf := bytes.NewBuffer(nil)

	data := ASGEventData{
		ASG:    a,
		Config: Conf,
	}

	err := t.Execute(buf, data)
	if err != nil {
		Log.Debug("Template generation error", err)
	}

	d.NewEvent("Reapable ASG Discovered", string(buf.Bytes()), nil, nil)
	Log.Debug("Reapable ASG Event posted to DataDog.")
}

func (d DataDog) NewReapableInstanceEvent(i *Instance) {
	var funcMap = template.FuncMap{
		"MakeTerminateLink": MakeTerminateLink,
		"MakeIgnoreLink":    MakeIgnoreLink,
		"MakeWhitelistLink": MakeWhitelistLink,
	}
	t := template.Must(template.New("reapable").Funcs(funcMap).Parse(reapableInstanceTemplateDataDog))
	buf := bytes.NewBuffer(nil)

	data := InstanceEventData{
		Instance: i,
		Config:   Conf,
	}

	err := t.Execute(buf, data)
	if err != nil {
		Log.Debug("Template generation error", err)
	}

	d.NewEvent("Reapable Instance Discovered", string(buf.Bytes()), nil, nil)
	Log.Debug("Reapable Instance Event posted to DataDog.")
}

const reapableInstanceTemplateDataDog = `%%%
Reaper has discovered an instance qualified as reapable: {{if .Instance.Name}}"{{.Instance.Name}}" {{end}}[{{.Instance.Id}}]({{.Instance.AWSConsoleURL}}) in region: [{{.Instance.Region}}](https://{{.Instance.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.Instance.Region}}).\n
{{if .Instance.Owned}}Owned by {{.Instance.Owner}}.\n{{end}}
State: {{.Instance.State}}.\n
{{ if .Instance.AWSConsoleURL}}{{.Instance.AWSConsoleURL}}\n{{end}}
[AWS Console URL]({{.Instance.AWSConsoleURL}})\n
[Whitelist this instance.]({{ MakeWhitelistLink .Config.TokenSecret .Config.HTTPApiURL .Instance.Region .Instance.Id }})
[Terminate this instance.]({{ MakeTerminateLink .Config.TokenSecret .Config.HTTPApiURL .Instance.Region .Instance.Id }})
%%%`

const reapableASGTemplateDataDog = `%%%
Reaper has discovered an ASG qualified as reapable: {{if .ASG.Name}}"{{.ASG.Name}}" {{end}}[{{.ASG.Id}}]({{.ASG.AWSConsoleURL}}) in region: [{{.ASG.Region}}](https://{{.ASG.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.ASG.Region}}).\n
{{if .ASG.Owned}}Owned by {{.ASG.Owner}}.\n{{end}}
State: {{.ASG.State}}.\n
{{ if .ASG.AWSConsoleURL}}{{.ASG.AWSConsoleURL}}\n{{end}}
[AWS Console URL]({{.ASG.AWSConsoleURL}})\n
[Whitelist this ASG.]({{ MakeWhitelistLink .Config.TokenSecret .Config.HTTPApiURL .ASG.Region .ASG.Id }})
[Terminate this ASG.]({{ MakeTerminateLink .Config.TokenSecret .Config.HTTPApiURL .ASG.Region .ASG.Id }})
%%%`
