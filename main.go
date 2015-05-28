package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/op/go-logging"
)

var (
	Log           *logging.Logger
	conf          *Config
	mailer        *Mailer
	dryrun        = false
	enableDataDog bool
	events        []EventReporter
)

func init() {
	var configFile string
	var logFile string

	flag.StringVar(&configFile, "conf", "", "path to config file")
	flag.StringVar(&logFile, "logfile", "", "path to log file")
	flag.BoolVar(&dryrun, "dryrun", false, "dry run, don't make changes")
	flag.BoolVar(&enableDataDog, "datadog", true, "enable DataDog reporting, requires dd-agent running")
	flag.Parse()

	// set up logging
	Log = logging.MustGetLogger("Reaper")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	format := logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} ▶%{color:reset} %{message}")
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)

	if logFile != "" {
		// open file write only, append mode
		// create it if it doesn't exist
		f, err := os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
		if err != nil {
			Log.Error("Unable to open logfile '%s'", logFile)
		}
		logFileFormat := logging.MustStringFormatter("15:04:05.000: %{shortfunc} ▶ %{level:.4s} ▶ %{message}")
		logFileBackend := logging.NewLogBackend(f, "", 0)
		logFileBackendFormatter := logging.NewBackendFormatter(logFileBackend, logFileFormat)
		logging.SetBackend(backendFormatter, logFileBackendFormatter)
	}

	if configFile == "" {
		Log.Error("Config file is a required Argument. Specify with -conf='filename'")
		os.Exit(1)
	}

	if c, err := LoadConfig(configFile); err == nil {
		conf = c
		Log.Info("Configuration loaded from %s", configFile)
		Log.Debug("SMTP Config: %s", conf.SMTP.String())
		Log.Debug("SMTP From: %s", conf.SMTP.From.Address.String())

	} else {
		Log.Error("toml", err)
		os.Exit(1)
	}

	if enableDataDog {
		Log.Info("DataDog enabled.")
		events = append(events, DataDog{})
	} else {
		events = append(events, NoEventReporter{})
	}

	mailer = NewMailer(*conf)

	if dryrun {
		Log.Notice("Dry run mode enabled, no changes will be made")
	}

}

func main() {
	reapRunner := NewReaper(*conf, mailer, events)
	if dryrun {
		reapRunner.DryRunOn()
	} else {
		reapRunner.DryRunOff()
	}

	// Run the reaper process
	reapRunner.Start()

	// run the HTTP server
	api := NewHTTPApi(*conf)
	if err := api.Serve(); err != nil {
		Log.Error(err.Error())
	} else {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		sig := <-c
		Log.Notice(fmt.Sprintf("Got signal %s, stopping services", sig.String()))
		Log.Notice("Stopping HTTP")
		api.Stop()
		Log.Notice("Stopping reaper runner")
		reapRunner.Stop()
	}
}
