package reapable

import "github.com/milescrabill/reaper/state"

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
	Terminable
	Stoppable
	Whitelistable
	Saveable
	ReapableDescription() string
}

// used to identify unowned resources
type UnownedError struct {
	ErrorText string
}

func (u UnownedError) Error() string {
	return u.ErrorText
}
