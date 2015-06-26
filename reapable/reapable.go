package reapable

import (
	"fmt"
	"sync"

	"github.com/milescrabill/reaper/filters"
	"github.com/milescrabill/reaper/state"
)

type Terminable interface {
	Terminate() (bool, error)
}

type Stoppable interface {
	Stop() (bool, error)
	ForceStop() (bool, error)
}

type Whitelistable interface {
	Whitelist() (bool, error)
}

type Saveable interface {
	Save(state *state.State) (bool, error)
	Unsave() (bool, error)
	ReaperState() *state.State
	IncrementState() bool
}

//                ,____
//                |---.\
//        ___     |    `
//       / .-\  ./=)
//      |  | |_/\/|
//      ;  |-;| /_|
//     / \_| |/ \ |
//    /      \/\( |
//    |   /  |` ) |
//    /   \ _/    |
//   /--._/  \    |
//   `/|)    |    /
//     /     |   |
//   .'      |   |
//  /         \  |
// (_.-.__.__./  /
// credit: jgs, http://www.chris.com/ascii/index.php?art=creatures/grim%20reapers

type Reapable interface {
	filters.Filterable
	Terminable
	Stoppable
	Whitelistable
	Saveable

	ReapableDescription() string
	ReapableDescriptionShort() string
	ReapableDescriptionTiny() string
}

type Region string

func (r Region) String() string {
	return string(r)
}

type ID string

func (i ID) String() string {
	return string(i)
}

type Reapables struct {
	sync.RWMutex
	storage map[Region]map[ID]Reapable
}

func NewReapables(regions []string) *Reapables {
	r := Reapables{}
	r.Lock()
	defer r.Unlock()

	// initialize Reapables map
	r.storage = make(map[Region]map[ID]Reapable)
	for _, region := range regions {
		r.storage[Region(region)] = make(map[ID]Reapable)
	}
	return &r
}

func (rs *Reapables) Put(region Region, id ID, r Reapable) {
	rs.Lock()
	defer rs.Unlock()
	rs.storage[region][id] = r
}

func (rs *Reapables) Get(region Region, id ID) (Reapable, error) {
	rs.RLock()
	defer rs.RUnlock()
	r, ok := rs.storage[region][id]
	if ok {
		return r, nil
	}
	return r, fmt.Errorf("Could not find %s", r.ReapableDescriptionTiny())
}

func (rs *Reapables) Delete(region Region, id ID) {
	rs.RLock()
	defer rs.RUnlock()
	delete(rs.storage[region], id)
}

type ReapableContainer struct {
	Reapable
	Region Region
	ID     ID
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
