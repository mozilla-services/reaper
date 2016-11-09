package reapable

import (
	"fmt"
	"sync"
)

// Singleton instance of Reapables
// Functions Get, Put, Delete, and All interact with singleton
var singleton Reapables

// Reapables is a container for instances of Reapable
// This package includes a singleton instance of Reapables
type Reapables struct {
	sync.RWMutex
	storage map[Region]map[ID]Reapable
}

func init() {
	regions := []string{
		"us-west-1",
		"us-west-2",
		"eu-west-1",
		"eu-central-1",
		"ap-northeast-2",
		"ap-southeast-2",
		"us-east-1",
		"us-east-2",
		"sa-east-1",
		"ap-southeast-1",
		"ap-northeast-1",
		"ap-south-1",
		"AWS GovCloud (US)",
	}

	singleton = Reapables{}
	singleton.Lock()
	defer singleton.Unlock()

	// initialize Reapables map
	singleton.storage = make(map[Region]map[ID]Reapable)
	for _, region := range regions {
		singleton.storage[Region(region)] = make(map[ID]Reapable)
	}
}

// Put adds a Reapable to the singleton Reapables
func Put(region Region, id ID, r Reapable) {
	singleton.Put(region, id, r)
}

// Get returns a Reapable from the singleton Reapables
func Get(region Region, id ID) (Reapable, error) {
	return singleton.Get(region, id)
}

// Delete deletes a Reapable from the singleton Reapables
func Delete(region Region, id ID) {
	singleton.Delete(region, id)
}

// All returns all Reapables in the singleton Reapables
func All() []Reapable {
	return singleton.All()
}

// Put adds a Reapable to Reapables
func (rs *Reapables) Put(region Region, id ID, r Reapable) {
	rs.Lock()
	defer rs.Unlock()
	rs.storage[region][id] = r
}

// Get returns a Reapable from Reapables
func (rs *Reapables) Get(region Region, id ID) (Reapable, error) {
	rs.RLock()
	defer rs.RUnlock()
	r, ok := rs.storage[region][id]
	if ok {
		return r, nil
	}
	return r, NotFoundError{fmt.Sprintf("Could not find resource %s in %s", id.String(), region.String())}
}

// Delete removes a Reapable from Reapables
func (rs *Reapables) Delete(region Region, id ID) {
	rs.Lock()
	defer rs.Unlock()
	delete(rs.storage[region], id)
}

// All returns an array of all Reapables
func (rs *Reapables) All() []Reapable {
	var reapables []Reapable
	rs.RLock()
	defer rs.RUnlock()
	for _, regionMap := range rs.storage {
		for _, reapable := range regionMap {
			reapables = append(reapables, reapable)
		}
	}
	return reapables
}

// UnownedError is used to identify unowned resources
type UnownedError struct {
	ErrorText string
}

func (u UnownedError) Error() string {
	return u.ErrorText
}

// NotFoundError is used when Reapables does not have a Reapable
type NotFoundError struct {
	ErrorText string
}

func (n NotFoundError) Error() string {
	return n.ErrorText
}
