package filters

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	log "github.com/mozilla-services/reaper/reaperlog"
)

type Filterable interface {
	Filter(Filter) bool
	AddFilterGroup(string, FilterGroup)
}

func ApplyFilters(f Filterable, fs FilterGroup) bool {
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

func FormatFiltersText(filters map[string]Filter) string {
	var filterText []string
	for _, filter := range filters {
		filterText = append(filterText, fmt.Sprintf("%s(%s)", filter.Function, strings.Join(filter.Arguments, ", ")))
	}
	// alphabetize and join filters
	sort.Strings(filterText)
	return strings.Join(filterText, ", ")
}

func FormatFilterGroupsText(filterGroups map[string]FilterGroup) string {
	var filterGroupText []string
	for name, filterGroup := range filterGroups {
		filterGroupText = append(filterGroupText, fmt.Sprintf("FilterGroup %s: [%s]", name, FormatFiltersText(filterGroup)))
	}
	return fmt.Sprintf("%s", strings.Join(filterGroupText, ", "))
}

type FilterGroup map[string]Filter

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
		log.Errorf("could not parse %s as int64", filter.Arguments[v])
		return 0, err
	}
	return i, nil
}

func (filter *Filter) BoolValue(v int) (bool, error) {
	b, err := strconv.ParseBool(filter.Arguments[v])
	if err != nil {
		log.Errorf("could not parse %s as bool", filter.Arguments[v])
		return false, err
	}
	return b, nil
}
