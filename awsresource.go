package reaper

import (
	"fmt"
	"net/mail"
	"strings"
	"time"
)

type StateEnum int

const (
	STATE_START StateEnum = iota
	STATE_NOTIFY1
	STATE_NOTIFY2
	STATE_IGNORE
)

func (s StateEnum) String() string {
	switch s {
	case STATE_NOTIFY1:
		return "notify1"
	case STATE_NOTIFY2:
		return "notify2"
	case STATE_IGNORE:
		return "ignore"
	default:
		return "start"
	}
}

type State struct {
	State StateEnum

	// State must be maintained until this time
	Until time.Time
}

func (s *State) String() string {
	return s.State.String() + s_sep + s.Until.Format(s_tformat)
}

func ParseState(state string) (defaultState *State) {

	defaultState = &State{STATE_START, time.Time{}}

	if state == "" {
		return
	}

	s := strings.Split(state, s_sep)

	if len(s) != 2 {
		return
	}

	var stateEnum StateEnum
	switch s[0] {
	case "start":
		stateEnum = STATE_START
	case "notify1":
		stateEnum = STATE_NOTIFY1
	case "notify2":
		stateEnum = STATE_NOTIFY2
	case "ignore":
		stateEnum = STATE_IGNORE
	default:
		return
	}

	t, err := time.Parse(s_tformat, s[1])
	if err != nil {
		return
	}

	return &State{stateEnum, t}
}

type AWSResources []*AWSResource
type AWSResource struct {
	id          string
	name        string
	region      string
	state       string
	description string
	vpc_id      string
	owner_id    string

	tags map[string]string

	// reaper state
	reaper *State
}

func (a *AWSResource) Tagged(tag string) bool {
	_, ok := a.tags[tag]
	return ok
}

func (a *AWSResource) Id() string     { return a.id }
func (a *AWSResource) Region() string { return a.region }
func (a *AWSResource) State() string  { return a.state }
func (a *AWSResource) Reaper() *State { return a.reaper }

// Tag returns the tag's value or an empty string if it does not exist
func (a *AWSResource) Tag(t string) string { return a.tags[t] }

func (a *AWSResource) Owned() bool { return a.Tagged("Owner") }

// Owner extracts useful information out of the Owner tag which should
// be parsable by mail.ParseAddress
func (a *AWSResource) Owner() *mail.Address {
	// properly formatted email
	if addr, err := mail.ParseAddress(a.Tag("Owner")); err == nil {
		return addr
	}

	// username -> mozilla.com email
	if addr, err := mail.ParseAddress(fmt.Sprintf("%s@mozilla.com", a.Tag("Owner"))); a.Tagged("Owner") && err == nil {
		return addr
	}

	return nil
}
