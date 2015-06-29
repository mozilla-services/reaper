package filters

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	log "github.com/milescrabill/reaper/reaperlog"
)

type Filterable interface {
	Filter(Filter) bool
}

func ApplyFilters(f Filterable, fs map[string]Filter) bool {
	// defaults to a match
	matched := true

	// if any of the filters return false -> not a match
	for _, filter := range fs {
		if !f.Filter(filter) {
			matched = false
		}
	}

	return matched
}

func PrintFilters(filters map[string]Filter) string {
	var filterText []string
	for _, filter := range filters {
		filterText = append(filterText, fmt.Sprintf("%s(%s)", filter.Function, strings.Join(filter.Arguments, ", ")))
	}
	// alphabetize and join filters
	sort.Strings(filterText)
	return strings.Join(filterText, ", ")
}

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
