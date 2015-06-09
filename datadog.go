package main

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/PagerDuty/godspeed"
)

// DataDogConfig is the configuration for a DataDog
type DataDogConfig struct {
	Enabled bool
}

// implements EventReporter, sends events and statistics to DataDog
// uses godspeed, requires dd-agent running
type DataDog struct {
	Config        *DataDogConfig
	eventTemplate template.Template
	godspeed      *godspeed.Godspeed
}

// TODO: make this async?
// TODO: don't recreate godspeed
func (d *DataDog) Godspeed() *godspeed.Godspeed {
	if d.godspeed == nil {
		g, err := godspeed.NewDefault()
		if err != nil {
			Log.Debug("Error creating Godspeed, ", err)
			return nil
		}
		d.godspeed = g
	}
	return d.godspeed
}

// NewEvent reports an event to DataDog
func (d *DataDog) NewEvent(title string, text string, fields map[string]string, tags []string) {
	g := d.Godspeed()
	// TODO: fix?
	// defer g.Conn.Close()
	err := g.Event(title, text, fields, tags)
	if err != nil {
		Log.Debug(fmt.Sprintf("Error reporting Godspeed event %s", title), err)
	}
	Log.Debug(fmt.Sprintf("Event %s posted to DataDog.", title))
}

// NewStatistic reports a gauge to DataDog
func (d *DataDog) NewStatistic(name string, value float64, tags []string) {
	g := d.Godspeed()

	// TODO: fix?
	// defer g.Conn.Close()
	err := g.Gauge(name, value, tags)
	if err != nil {
		Log.Debug(fmt.Sprintf("Error reporting Godspeed statistic %s", name), err)
	}

	Log.Debug(fmt.Sprintf("Statistic %s posted to DataDog.", name))
}

// NewCountStatistic reports an Incr to DataDog
func (d *DataDog) NewCountStatistic(name string, tags []string) {
	g := d.Godspeed()

	// TODO: fix?
	// defer g.Conn.Close()
	err := g.Incr(name, tags)
	if err != nil {
		Log.Debug(fmt.Sprintf("Error reporting Godspeed incr %s", name), err)
	}

	Log.Debug(fmt.Sprintf("Statistic %s posted to DataDog.", name))
}

// funcMap maps strings to functions for templates
var funcMap = template.FuncMap{
	"MakeTerminateLink": MakeTerminateLink,
	"MakeIgnoreLink":    MakeIgnoreLink,
	"MakeWhitelistLink": MakeWhitelistLink,
	"MakeStopLink":      MakeStopLink,
	"MakeForceStopLink": MakeForceStopLink,
}

// NewReapableASGEvent reports an event to DataDog about a specific
// reapable ASG
func (d DataDog) NewReapableASGEvent(a *AutoScalingGroup) {
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
	Log.Debug(fmt.Sprintf("Event Reapable ASG (%s) posted to DataDog.", a.ID))
}

// NewReapableInstanceEvent reports an event to DataDog about a specific
// reapable Instance
func (d DataDog) NewReapableInstanceEvent(i *Instance) {
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

	d.NewEvent(fmt.Sprintf("Reapable Instance %s Discovered", i.ID), string(buf.Bytes()), nil, nil)
	Log.Debug(fmt.Sprintf("Event Reapable Instance (%s) posted to DataDog.", i.ID))
}

const reapableInstanceTemplateDataDog = `%%%
Reaper has discovered an instance qualified as reapable: {{if .Instance.Name}}"{{.Instance.Name}}" {{end}}[{{.Instance.ID}}]({{.Instance.AWSConsoleURL}}) in region: [{{.Instance.Region}}](https://{{.Instance.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.Instance.Region}}).\n
{{if .Instance.Owned}}Owned by {{.Instance.Owner}}.\n{{end}}
State: {{ .Instance.State.String}}.\n
Instance Type: {{ .Instance.InstanceType}}.\n
{{ if .Instance.PublicIPAddress.String}}This instance's public IP: {{.Instance.PublicIPAddress}}\n{{end}}
{{ if .Instance.AWSConsoleURL}}{{.Instance.AWSConsoleURL}}\n{{end}}
[AWS Console URL]({{.Instance.AWSConsoleURL}})\n
[Whitelist]({{ MakeWhitelistLink .Config.TokenSecret .Config.HTTPApiURL .Instance.Region .Instance.ID }}) this instance.
[Stop]({{ MakeStopLink .Config.TokenSecret .Config.HTTPApiURL .Instance.Region .Instance.ID }}) this instance.
[Terminate]({{ MakeTerminateLink .Config.TokenSecret .Config.HTTPApiURL .Instance.Region .Instance.ID }}) this instance.
%%%`

const reapableASGTemplateDataDog = `%%%
Reaper has discovered an ASG qualified as reapable: [{{.ASG.ID}}]({{.ASG.AWSConsoleURL}}) in region: [{{.ASG.Region}}](https://{{.ASG.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.ASG.Region}}).\n
{{if .ASG.Owned}}Owned by {{.ASG.Owner}}.\n{{end}}
{{ if .ASG.AWSConsoleURL}}{{.ASG.AWSConsoleURL}}\n{{end}}
[AWS Console URL]({{.ASG.AWSConsoleURL}})\n
[Whitelist]({{ MakeWhitelistLink .Config.TokenSecret .Config.HTTPApiURL .ASG.Region .ASG.ID }}) this ASG.
[Terminate]({{ MakeTerminateLink .Config.TokenSecret .Config.HTTPApiURL .ASG.Region .ASG.ID }}) this ASG.\n
[Scale]({{ MakeStopLink .Config.TokenSecret .Config.HTTPApiURL .ASG.Region .ASG.ID }}) this ASG to 0 instances
[Force Scale]({{ MakeForceStopLink .Config.TokenSecret .Config.HTTPApiURL .ASG.Region .ASG.ID }}) this ASG to 0 instances (changes minimum)
%%%`
