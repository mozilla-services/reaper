package filters

import (
	"fmt"
	"strconv"

	log "github.com/mostlygeek/reaper/reaperlog"
)

type Filter struct {
	Function  string
	Arguments []string
}

func NewFilter(f string, args []string) *Filter {
	return &Filter{
		Function:  f,
		Arguments: args,
	}
}

func (filter *Filter) Int64Value(v int) (int64, error) {
	// parseint -> base 10, 64 bit int
	i, err := strconv.ParseInt(filter.Arguments[v], 10, 64)
	if err != nil {
		log.Error(fmt.Sprintf("could not parse %s as int64", filter.Arguments[v]))
		return 0, err
	}
	return i, nil
}

func (filter *Filter) BoolValue(v int) (bool, error) {
	b, err := strconv.ParseBool(filter.Arguments[v])
	if err != nil {
		log.Error(fmt.Sprintf("could not parse %s as bool", filter.Arguments[v]))
		return false, err
	}
	return b, nil
}
