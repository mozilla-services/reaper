package state

import (
	"strings"
	"time"
)

const (
	InitialState StateEnum = iota
	FirstState
	SecondState
	ThirdState
	FinalState
	IgnoreState
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) (err error) {
	d.Duration, err = time.ParseDuration(string(text))
	return
}

type StatesConfig struct {
	Interval            Duration
	FirstStateDuration  Duration
	SecondStateDuration Duration
	ThirdStateDuration  Duration
}

type StateEnum int

// StateEnum.String() in stateenum_string.go

type State struct {
	State StateEnum

	reaperTagTimeFormat string
	reaperTagSeparator  string

	Updated bool

	// State must be maintained until this time
	Until time.Time
}

func (s *State) FinalStateTime(c StatesConfig) time.Time {
	switch s.State {
	case FirstState:
		return s.Until.Add(c.SecondStateDuration.Duration).Add(c.ThirdStateDuration.Duration)
	case SecondState:
		return s.Until.Add(c.ThirdStateDuration.Duration)
	case ThirdState:
		return s.Until
	default:
		// IgnoreState and FinalState
		return time.Now()
	}
}

func (s *State) String() string {
	return s.State.String() + s.reaperTagSeparator + s.Until.Format(s.reaperTagTimeFormat)
}

func NewState() *State {
	// default
	return &State{
		State:               InitialState,
		Until:               time.Now(),
		reaperTagSeparator:  "|",
		reaperTagTimeFormat: "2006-01-02 03:04PM MST",
	}
}

func NewStateWithUntil(until time.Time) *State {
	// default
	return &State{
		State:               InitialState,
		Until:               until,
		reaperTagSeparator:  "|",
		reaperTagTimeFormat: "2006-01-02 03:04PM MST",
	}
}

func NewStateWithUntilAndState(until time.Time, state StateEnum) *State {
	// default
	return &State{
		State:               state,
		Until:               until,
		reaperTagSeparator:  "|",
		reaperTagTimeFormat: "2006-01-02 03:04PM MST",
	}
}

func NewStateWithTag(state string) *State {
	if state == "" {
		return NewState()
	}

	s := strings.Split(state, NewState().reaperTagSeparator)

	if len(s) != 2 {
		return NewState()
	}

	var stateEnum StateEnum
	switch s[0] {
	case "FirstState":
		stateEnum = FirstState
	case "SecondState":
		stateEnum = SecondState
	case "ThirdState":
		stateEnum = ThirdState
	case "FinalState":
		stateEnum = FinalState
	case "IgnoreState":
		stateEnum = IgnoreState
	}

	// TODO: this only accepts one time format...
	t, err := time.Parse(NewState().reaperTagTimeFormat, s[1])
	if err != nil {
		return NewState()
	}

	return NewStateWithUntilAndState(t, stateEnum)
}
