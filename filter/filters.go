package filter

import (
	"time"

	"github.com/mostlygeek/reaper/aws"
)

func Owned(i *aws.Instance) bool    { return i.Owned() }
func NotOwned(i *aws.Instance) bool { return !Owned(i) }

func AutoScaled(i *aws.Instance) bool    { return i.AutoScaled() }
func NotAutoscaled(i *aws.Instance) bool { return !AutoScaled(i) }

//func TimeoutExpired(i *aws.Instance) bool { }

func Id(id string) aws.FilterFunc {
	return func(i *aws.Instance) bool {
		return i.Id() == id
	}
}

func Not(f aws.FilterFunc) aws.FilterFunc {
	return func(i *aws.Instance) bool {
		return !f(i)
	}
}

func Tagged(tag string) aws.FilterFunc {
	return func(i *aws.Instance) bool {
		return i.Tagged(tag)
	}
}

func Running(i *aws.Instance) bool {
	return i.State() == "running"
}

// ReaperReady creates a FilterFunc that checks if the instance is qualified
// additional reaper work
func ReaperReady(runningTime time.Duration) aws.FilterFunc {
	return func(i *aws.Instance) bool {
		if i.Reaper().State == aws.STATE_START {
			return i.LaunchTime().Add(runningTime).Before(time.Now())
		} else {
			return i.Reaper().Until.Before(time.Now())
		}
	}
}
