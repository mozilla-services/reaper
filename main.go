package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/op/go-logging"
)

// Log, Events, Reapables, and Conf are all exported global variables

var (
	Log       *logging.Logger
	Events    []EventReporter
	Reapables map[string]map[string]Reapable
	Conf      *Config
	mailer    *Mailer
)

func init() {
	configFile := flag.String("config", "", "path to config file")
	dryRun := flag.Bool("dryrun", true, "dry run, don't make changes")
	flag.Parse()

	// set up logging
	Log = logging.MustGetLogger("Reaper")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	format := logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} ▶%{color:reset} %{message}")
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)

	if *configFile == "" {
		Log.Error("Config file is a required Argument. Specify with -conf='filename'")
		os.Exit(1)
	}

	if c, err := LoadConfig(*configFile); err == nil {
		Conf = c
		Log.Info("Configuration loaded from %s", *configFile)
		Log.Debug("SMTP Config: %s", Conf.SMTP.String())
		Log.Debug("SMTP From: %s", Conf.SMTP.From.Address.String())

	} else {
		Log.Error("toml", err)
		os.Exit(1)
	}

	if Conf.LogFile != "" {
		// open file write only, append mode
		// create it if it doesn't exist
		f, err := os.OpenFile(Conf.LogFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
		if err != nil {
			Log.Error("Unable to open logfile '%s'", Conf.LogFile)
		} else {
			Log.Info("Logging to %s", Conf.LogFile)
			logFileFormat := logging.MustStringFormatter("15:04:05.000: %{shortfunc} ▶ %{level:.4s} ▶ %{message}")
			logFileBackend := logging.NewLogBackend(f, "", 0)
			logFileBackendFormatter := logging.NewBackendFormatter(logFileBackend, logFileFormat)
			logging.SetBackend(backendFormatter, logFileBackendFormatter)
		}
	}

	if Conf.Events.DataDog {
		Log.Info("DataDog EventReporter enabled.")
		Events = append(Events, DataDog{})
	}

	if Conf.Events.Email {
		Log.Info("Email EventReporter enabled.")
		Events = append(Events, NewMailer(*Conf))
	}

	Conf.DryRun = *dryRun
	if *dryRun {
		Log.Notice("Dry run mode enabled, no changes will be made")
	}

	// initialize reapables map
	// must initialize submaps or else -> nil map crashes
	Reapables = make(map[string]map[string]Reapable)
	regions := Conf.AWS.Regions
	for _, region := range regions {
		Reapables[region] = make(map[string]Reapable)
	}

}

func main() {
	reapRunner := NewReaper(*Conf)

	// Run the reaper process
	reapRunner.Start()

	// run the HTTP server
	api := NewHTTPApi(*Conf)
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
