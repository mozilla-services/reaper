package reaper

import (
	"fmt"
	"net/mail"
)

type AWSResources []AWSResource
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
func (a *AWSResource) Name() string   { return a.name }
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

func (as AWSResources) Owned() {
	var bs AWSResources
	for i := 0; i < len(as); i++ {
		if as[i].Owned() {
			bs = append(bs, as[i])
		}
	}
	as = bs
}

func (as AWSResources) Tagged(tag string) {
	var bs AWSResources
	for i := 0; i < len(as); i++ {
		if as[i].Tagged(tag) {
			bs = append(bs, as[i])
		}
	}
	as = bs
}
