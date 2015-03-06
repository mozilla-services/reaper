package reaper

import (
	"fmt"
	"testing"
)

var (
	_ = fmt.Println
)

func TestParseState(t *testing.T) {
	in := (&State{}).String()

	if ParseState(in).String() != in {
		t.Error()
	}
}

func TestParseInvalid(t *testing.T) {
	expected := (&State{}).String() // the default values

	// should all be unparseable
	a := []string{
		"start|2015-01-24 25PM MST",
		"delay|2015-01-24 19PM",
	}

	for _, test := range a {
		s := ParseState(test)
		if s.String() != expected {
			t.Errorf("Failed on: %s", test)
		}
	}
}
