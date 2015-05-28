package reaper

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/PagerDuty/godspeed"
	. "github.com/tj/go-debug"
)

var debugEvents = Debug("reaper:events")

type EventReporter interface {
	NewEvent(title string, text string, fields map[string]string, tags []string)
	NewStatistic(name string, value float64, tags []string)
	NewReapableInstanceEvent(i *Instance)
}

type DataDog struct {
	eventTemplate template.Template
}

// TODO: make this async?
// TODO: not recreate godspeed
func (d DataDog) NewEvent(title string, text string, fields map[string]string, tags []string) {
	g, err := godspeed.NewDefault()
	if err != nil {
		debugEvents("Error creating Godspeed, ", err)
	}
	defer g.Conn.Close()
	err = g.Event(title, text, fields, tags)
	if err != nil {
		debugEvents(fmt.Sprintf("Error reporting Godspeed event %s", title), err)
	}
}

func (d DataDog) NewStatistic(name string, value float64, tags []string) {
	g, err := godspeed.NewDefault()
	if err != nil {
		debugEvents("Error creating Godspeed, ", err)
	}
	defer g.Conn.Close()
	err = g.Gauge(name, value, tags)
	if err != nil {
		debugEvents(fmt.Sprintf("Error reporting Godspeed statistic %s", name), err)
	}
}

func (d DataDog) NewReapableInstanceEvent(i *Instance) {
	t := template.Must(template.New("reapable").Parse(reapableInstanceTemplate))
	buf := bytes.NewBuffer(nil)

	err := t.Execute(buf, i)
	if err != nil {
		debugEvents("Template generation error", err)
	}

	d.NewEvent("Reapable Instance Discovered", string(buf.Bytes()), nil, nil)
}

const reapableInstanceTemplate = `Reaper has discovered a new reapable instance: {{if .Name}}"{{.Name}}" {{end}}{{.Id}} in region {{.Region}}.
{{if .Owned}}Owned by {{.Owner}}{{end}}
State: {{.State}}.
Check it out [here]({{.AWSConsoleURL}}).
`
