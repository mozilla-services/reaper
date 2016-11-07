package reapable

import (
	"net/mail"

	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/state"
)

type Saveable interface {
	Save(state *state.State) (bool, error)
	Unsave() (bool, error)
	ReaperState() *state.State
	IncrementState() bool
	SetUpdated(bool)
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
	Terminate() (bool, error)
	Stop() (bool, error)
	Whitelist() (bool, error)
	Saveable

	Owner() *mail.Address
	ID() ID
	Region() Region

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
