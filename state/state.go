package state

import (
	"strings"
	"time"
)

const (
	STATE_START StateEnum = iota
	STATE_NOTIFY1
	STATE_NOTIFY2
	STATE_REAPABLE
	STATE_WHITELIST
)

type StateEnum int

// StateEnum.String() in stateenum_string.go

type State struct {
	State StateEnum

	reaperTagTimeFormat string
	reaperTagSeparator  string

	// State must be maintained until this time
	Until time.Time
}

func (s *State) String() string {
	return s.State.String() + s.reaperTagSeparator + s.Until.Format(s.reaperTagTimeFormat)
}

func NewState() *State {
	// default
	return &State{
		State:               STATE_START,
		Until:               time.Now(),
		reaperTagSeparator:  "|",
		reaperTagTimeFormat: "2006-01-02 03:04PM MST",
	}
}

func NewStateWithUntil(until time.Time) *State {
	// default
	return &State{
		State:               STATE_START,
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
	defaultState := NewState()

	if state == "" {
		return defaultState
	}

	s := strings.Split(state, defaultState.reaperTagSeparator)

	if len(s) != 2 {
		return defaultState
	}

	var stateEnum StateEnum
	switch s[0] {
	case "STATE_START":
		stateEnum = STATE_START
	case "STATE_NOTIFY1":
		stateEnum = STATE_NOTIFY1
	case "STATE_NOTIFY2":
		stateEnum = STATE_NOTIFY2
	case "STATE_WHITELIST":
		stateEnum = STATE_WHITELIST
	case "STATE_REAPABLE":
		stateEnum = STATE_REAPABLE
	default:
		return defaultState
	}

	// TODO: this only accepts one time format...
	t, err := time.Parse(defaultState.reaperTagTimeFormat, s[1])
	if err != nil {
		return defaultState
	}

	defaultState.Until = t
	defaultState.State = stateEnum
	return defaultState
}
