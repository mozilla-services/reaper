package filters

import (
	"os"
	"strconv"

	"github.com/op/go-logging"
)

var Log *logging.Logger

func init() {
	// set up logging
	Log = logging.MustGetLogger("Reaper")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	format := logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} ▶%{color:reset} %{message}")
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)
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
		Log.Error("could not parse %s as int64", filter.Arguments[v])
		return 0, err
	}
	return i, nil
}

func (filter *Filter) BoolValue(v int) (bool, error) {
	b, err := strconv.ParseBool(filter.Arguments[v])
	if err != nil {
		Log.Error("could not parse %s as bool", filter.Arguments[v])
		return false, err
	}
	return b, nil
}
