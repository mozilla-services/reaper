package filter

import (
	"github.com/mostlygeek/reaper"
)

func Owned(i *reaper.Instance) bool    { return i.Owned() }
func NotOwned(i *reaper.Instance) bool { return !Owned(i) }

func AutoScaled(i *reaper.Instance) bool    { return i.AutoScaled() }
func NotAutoscaled(i *reaper.Instance) bool { return !AutoScaled(i) }

//func TimeoutExpired(i *reaper.Instance) bool { }

func Id(id string) reaper.FilterFunc {
	return func(i *reaper.Instance) bool {
		return i.Id() == id
	}
}

func Not(f reaper.FilterFunc) reaper.FilterFunc {
	return func(i *reaper.Instance) bool {
		return !f(i)
	}
}

func Tagged(tag string) reaper.FilterFunc {
	return func(i *reaper.Instance) bool {
		return i.Tagged(tag)
	}
}

func Running(i *reaper.Instance) bool {
	return i.State() == "running"
}
