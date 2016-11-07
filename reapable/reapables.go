package reapable

import (
	"fmt"
	"sync"
)

var rs Reapables

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

	rs = Reapables{}
	rs.Lock()
	defer rs.Unlock()

	// initialize Reapables map
	rs.storage = make(map[Region]map[ID]Reapable)
	for _, region := range regions {
		rs.storage[Region(region)] = make(map[ID]Reapable)
	}
}

func Put(region Region, id ID, r Reapable) {
	rs.Put(region, id, r)
}

func (rs *Reapables) Put(region Region, id ID, r Reapable) {
	rs.Lock()
	defer rs.Unlock()
	rs.storage[region][id] = r
}

func Get(region Region, id ID) (Reapable, error) {
	return rs.Get(region, id)
}

func Delete(region Region, id ID) {
	rs.Delete(region, id)
}

func Iter() <-chan ReapableContainer {
	return rs.Iter()
}

func (rs *Reapables) Get(region Region, id ID) (Reapable, error) {
	rs.RLock()
	defer rs.RUnlock()
	r, ok := rs.storage[region][id]
	if ok {
		return r, nil
	}
	return r, NotFoundError{fmt.Sprintf("Could not find resource %s in %s", id.String(), region.String())}
}

func (rs *Reapables) Delete(region Region, id ID) {
	rs.RLock()
	defer rs.RUnlock()
	delete(rs.storage[region], id)
}

type ReapableContainer struct {
	Reapable
	region Region
	id     ID
}

func (r *ReapableContainer) Region() Region {
	return r.region
}

func (r *ReapableContainer) ID() ID {
	return r.id
}

func (rs *Reapables) Iter() <-chan ReapableContainer {
	ch := make(chan ReapableContainer)
	go func(c chan ReapableContainer) {
		rs.Lock()
		defer rs.Unlock()
		for region, regionMap := range rs.storage {
			for id, r := range regionMap {
				c <- ReapableContainer{r, region, id}
			}
		}
		close(ch)
	}(ch)
	return ch
}

// used to identify unowned resources
type UnownedError struct {
	ErrorText string
}

func (u UnownedError) Error() string {
	return u.ErrorText
}

type NotFoundError struct {
	ErrorText string
}

func (n NotFoundError) Error() string {
	return n.ErrorText
}
