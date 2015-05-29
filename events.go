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
}

// implements EventReporter but does nothing
type NoEventReporter struct {
}

func (n NoEventReporter) NewEvent(title string, text string, fields map[string]string, tags []string) {
}

func (n NoEventReporter) NewStatistic(name string, value float64, tags []string) {
}

func (n NoEventReporter) NewReapableInstanceEvent(i *Instance) {
}

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

type ReapableInstanceEventData struct {
	Instance *Instance
	Config   *Config
}

func (d DataDog) NewReapableInstanceEvent(i *Instance) {
	var funcMap = template.FuncMap{
		"MakeTerminateLink": MakeTerminateLink,
		"MakeIgnoreLink":    MakeIgnoreLink,
		"MakeWhitelistLink": MakeWhitelistLink,
	}
	t := template.Must(template.New("reapable").Funcs(funcMap).Parse(reapableInstanceTemplateDataDog))
	buf := bytes.NewBuffer(nil)

	data := ReapableInstanceEventData{
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
Reaper has discovered a new reapable instance: {{if .Instance.Name}}"{{.Instance.Name}}" {{end}}[{{.Instance.Id}}]({{.Instance.AWSConsoleURL}}) in region: [{{.Instance.Region}}](https://{{.Instance.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.Instance.Region}}).\n
{{if .Instance.Owned}}Owned by {{.Instance.Owner}}.\n{{end}}
State: {{.Instance.State}}.\n
{{ if .Instance.AWSConsoleURL}}{{.Instance.AWSConsoleURL}}\n{{end}}
[AWS Console URL]({{.Instance.AWSConsoleURL}})\n
[Whitelist this instance.]({{ MakeWhitelistLink .Config.TokenSecret .Config.HTTPApiURL .Instance.Region .Instance.Id }})
[Terminate this instance.]({{ MakeTerminateLink .Config.TokenSecret .Config.HTTPApiURL .Instance.Region .Instance.Id }})
%%%`
