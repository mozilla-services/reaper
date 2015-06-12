package aws

import (
	"bytes"
	"fmt"
	"net/url"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/mostlygeek/reaper/events"
	"github.com/mostlygeek/reaper/filters"
	"github.com/mostlygeek/reaper/state"
)

type AutoScalingGroup struct {
	AWSResource

	// autoscaling.Instance exposes minimal info
	Instances []string

	AutoScalingGroupARN     string
	CreatedTime             time.Time
	MaxSize                 int64
	MinSize                 int64
	DesiredCapacity         int64
	LaunchConfigurationName string
}

func NewAutoScalingGroup(region string, asg *autoscaling.Group) *AutoScalingGroup {
	a := AutoScalingGroup{
		AWSResource: AWSResource{
			ID:          *asg.AutoScalingGroupName,
			Name:        *asg.AutoScalingGroupName,
			Region:      region,
			Tags:        make(map[string]string),
			reaperState: state.NewStateWithUntil(time.Now().Add(config.Notifications.FirstNotification.Duration)),
		},
		AutoScalingGroupARN:     *asg.AutoScalingGroupARN,
		CreatedTime:             *asg.CreatedTime,
		MaxSize:                 *asg.MaxSize,
		MinSize:                 *asg.MinSize,
		DesiredCapacity:         *asg.DesiredCapacity,
		LaunchConfigurationName: *asg.LaunchConfigurationName,
	}

	for i := 0; i < len(asg.Instances); i++ {
		a.Instances = append(a.Instances, *asg.Instances[i].InstanceID)
	}

	for i := 0; i < len(asg.Tags); i++ {
		a.Tags[*asg.Tags[i].Key] = *asg.Tags[i].Value
	}

	return &a
}

func (a *AutoScalingGroup) ReapableEventText() *bytes.Buffer {
	t := template.Must(template.New("reapable-asg").Parse(reapableASGEventText))
	buf := bytes.NewBuffer(nil)

	data := struct {
		Config           *events.HTTPConfig
		AutoScalingGroup *AutoScalingGroup
	}{
		AutoScalingGroup: a,
		Config:           &config.HTTP,
	}
	err := t.Execute(buf, data)
	if err != nil {
		log.Debug("Template generation error", err)
	}
	return buf
}

const reapableASGEventText = `%%%
Reaper has discovered an ASG qualified as reapable: [{{.ASG.ID}}]({{.ASG.AWSConsoleURL}}) in region: [{{.ASG.Region}}](https://{{.ASG.Region}}.console.aws.amazon.com/ec2/v2/home?region={{.ASG.Region}}).\n
{{if .ASG.Owned}}Owned by {{.ASG.Owner}}.\n{{end}}
{{ if .ASG.AWSConsoleURL}}{{.ASG.AWSConsoleURL}}\n{{end}}
[AWS Console URL]({{.ASG.AWSConsoleURL}})\n
[Whitelist]({{ MakeWhitelistLink .Config.TokenSecret .Config.HTTPApiURL .ASG.Region .ASG.ID }}) this ASG.
[Terminate]({{ MakeTerminateLink .Config.TokenSecret .Config.HTTPApiURL .ASG.Region .ASG.ID }}) this ASG.\n
[Scale]({{ MakeStopLink .Config.TokenSecret .Config.HTTPApiURL .ASG.Region .ASG.ID }}) this ASG to 0 instances
[Force Scale]({{ MakeForceStopLink .Config.TokenSecret .Config.HTTPApiURL .ASG.Region .ASG.ID }}) this ASG to 0 instances (changes minimum)
%%%`

func (a *AutoScalingGroup) SizeGreaterThanOrEqualTo(size int64) bool {
	return a.DesiredCapacity >= size
}

func (a *AutoScalingGroup) SizeLessThanOrEqualTo(size int64) bool {
	return a.DesiredCapacity <= size
}

func (a *AutoScalingGroup) SizeEqualTo(size int64) bool {
	return a.DesiredCapacity == size
}

func (a *AutoScalingGroup) SizeLessThan(size int64) bool {
	return a.DesiredCapacity < size
}

func (a *AutoScalingGroup) SizeGreaterThan(size int64) bool {
	return a.DesiredCapacity <= size
}

func (a *AutoScalingGroup) Filter(filter filters.Filter) bool {
	matched := false
	// map function names to function calls
	switch filter.Function {
	case "SizeGreaterThan":
		if i, err := filter.Int64Value(0); err == nil && a.SizeGreaterThan(i) {
			matched = true
		}
	case "SizeLessThan":
		if i, err := filter.Int64Value(0); err == nil && a.SizeLessThan(i) {
			matched = true
		}
	case "SizeEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.SizeEqualTo(i) {
			matched = true
		}
	case "SizeLessThanOrEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.SizeLessThanOrEqualTo(i) {
			matched = true
		}
	case "SizeGreaterThanOrEqualTo":
		if i, err := filter.Int64Value(0); err == nil && a.SizeGreaterThanOrEqualTo(i) {
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
	default:
		log.Error("No function %s could be found for filtering ASGs.", filter.Function)
	}
	return matched
}

func (a *AutoScalingGroup) AWSConsoleURL() *url.URL {
	url, err := url.Parse(fmt.Sprintf("https://%s.console.aws.amazon.com/ec2/autoscaling/home?region=%s#AutoScalingGroups:ID=%s",
		a.Region, a.Region, a.ID))
	if err != nil {
		log.Error(fmt.Sprintf("Error generating AWSConsoleURL. %s", err))
	}
	return url
}

func (a *AutoScalingGroup) scaleToSize(force bool, size int64) (bool, error) {
	log.Debug("Stopping ASG %s in region %s", a.ID, a.Region)
	as := autoscaling.New(&aws.Config{Region: a.Region})

	input := &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: &a.ID,
		DesiredCapacity:      &size,
	}

	if force {
		input.MinSize = &size
	}

	_, err := as.UpdateAutoScalingGroup(input)
	if err != nil {
		log.Error("could not update ASG %s in region %s", a.ID, a.Region)
		return false, err
	}
	return true, nil
}

// TODO
func (a *AutoScalingGroup) Terminate() (bool, error) {
	log.Debug("Terminating ASG %s in region %s.", a.ID, a.Region)
	return false, nil
}

// Stop scales ASGs to 0
func (a *AutoScalingGroup) Stop() (bool, error) {
	// force -> false
	return a.scaleToSize(false, 0)
}

// ForceStop force scales ASGs to 0
func (a *AutoScalingGroup) ForceStop() (bool, error) {
	// force -> true
	return a.scaleToSize(true, 0)
}
