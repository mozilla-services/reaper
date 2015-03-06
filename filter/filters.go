package filter

import (
	"github.com/mostlygeek/reaper"
)

func Owned(i *reaper.Instance) bool    { return i.Owned() }
func NotOwned(i *reaper.Instance) bool { return !Owned(i) }

func AutoScaled(i *reaper.Instance) bool    { return i.AutoScaled() }
func NotAutoscaled(i *reaper.Instance) bool { return !AutoScaled(i) }
