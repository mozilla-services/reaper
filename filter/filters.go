package filter

import "time"

type FilterableInstance interface {
	Id() string
	Region() string
	State() string
	Owned() bool
	LaunchTime() time.Time
	Tagged(string) bool

	ReaperVisible() bool
	ReaperStarted() bool
	ReaperNotified(int) bool
	ReaperIgnored() bool
}

type FilterFunc func(FilterableInstance) bool

func Owned(i FilterableInstance) bool    { return i.Owned() }
func NotOwned(i FilterableInstance) bool { return !i.Owned() }

func AutoScaled(i FilterableInstance) bool {
	return i.Tagged("aws:autoscaling:groupName")
}
func NotAutoscaled(i FilterableInstance) bool { return !AutoScaled(i) }

func Id(id string) FilterFunc {
	return func(i FilterableInstance) bool {
		return i.Id() == id
	}
}

func Not(f FilterFunc) FilterFunc {
	return func(i FilterableInstance) bool {
		return !f(i)
	}
}

func Tagged(tag string) FilterFunc {
	return func(i FilterableInstance) bool {
		return i.Tagged(tag)
	}
}

func LaunchTimeEqual(time time.Time) FilterFunc {
	return func(i FilterableInstance) bool {
		return i.LaunchTime().Equal(time)
	}
}

func LaunchTimeAfter(time time.Time) FilterFunc {
	return func(i FilterableInstance) bool {
		return i.LaunchTime().After(time)
	}
}

func LaunchTimeBefore(time time.Time) FilterFunc {
	return func(i FilterableInstance) bool {
		return i.LaunchTime().Before(time)
	}
}

func Running(i FilterableInstance) bool {
	return i.State() == "running"
}

// ReaperReady creates a FilterFunc that checks if the instance is qualified
// additional reaper work
func ReaperReady(runningTime time.Duration) FilterFunc {
	return func(i FilterableInstance) bool {
		if i.ReaperStarted() {
			return i.LaunchTime().Add(runningTime).Before(time.Now())
		} else {
			return i.ReaperVisible()
		}
	}
}
