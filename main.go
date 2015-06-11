package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/mostlygeek/reaper/events"
	"github.com/op/go-logging"
)

// Log, Events, Reapables, and Conf are all exported global variables

var (
	// Log -> exported global logger
	Log *logging.Logger
	// Events -> exported global events array
	Events []events.EventReporter
	// Reapables -> exported global array of reapables
	Reapables map[string]map[string]Reapable
	// Conf -> exported global config
	Conf *Config
)

func init() {
	configFile := flag.String("config", "", "path to config file")
	dryRun := flag.Bool("dryrun", true, "dry run, don't make changes")
	interactive := flag.Bool("interactive", false, "interactive mode, reap based on prompt")
	flag.Parse()

	// set up logging
	Log = logging.MustGetLogger("Reaper")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	format := logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} ▶%{color:reset} %{message}")
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)

	// if no config file -> exit with error
	if *configFile == "" {
		Log.Error("Config file is a required Argument. Specify with -conf='filename'")
		os.Exit(1)
	}

	// if config file path specified, attempt to load it
	if c, err := LoadConfig(*configFile); err == nil {
		// catches panics loading config
		defer func() {
			if r := recover(); r != nil {
				Log.Error("Invalid config, %s", r)
				os.Exit(1)
			}
		}()

		// TODO: extraneous assignment?
		Conf = c
		Log.Info("Configuration loaded from %s", *configFile)

		// these methods have pointer receivers
		Log.Debug("SMTP Config: %s", &Conf.Events.Email)
		Log.Debug("SMTP From: %s", &Conf.Events.Email.From)

	} else {
		// config not successfully loaded -> exit with error
		Log.Error("toml", err)
		os.Exit(1)
	}

	// if log file path specified, attempt to load it
	if Conf.LogFile != "" {
		// open file write only, append mode
		// create it if it doesn't exist
		f, err := os.OpenFile(Conf.LogFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
		if err != nil {
			Log.Error("Unable to open logfile '%s'", Conf.LogFile)
		} else {
			// if the file was successfully opened
			Log.Info("Logging to %s", Conf.LogFile)
			// reconfigure logging with stdout and logfile as outputs
			logFileFormat := logging.MustStringFormatter("15:04:05.000: %{shortfunc} ▶ %{level:.4s} ▶ %{message}")
			logFileBackend := logging.NewLogBackend(f, "", 0)
			logFileBackendFormatter := logging.NewBackendFormatter(logFileBackend, logFileFormat)
			logging.SetBackend(backendFormatter, logFileBackendFormatter)
		}
	}

	if Conf.Events.DataDog.Enabled {
		Log.Info("DataDog EventReporter enabled.")
		Events = append(Events, &events.DataDog{
			Config: &Conf.Events.DataDog,
		})
	}

	// Events = append(Events, &events.ErrorEventReporter{})

	if Conf.Events.Email.Enabled {
		Log.Info("Email EventReporter enabled.")
		Events = append(Events, events.NewMailer(&Conf.SMTP))
	}

	if Conf.Events.Tagger.Enabled {
		Log.Info("Tagger EventReporter enabled.")
		Events = append(Events, &Tagger{
			Config: &Conf.Events.Tagger,
		})
	}

	// interactive mode and automatic reaping mode are mutually exclusive
	Conf.Interactive = *interactive
	if *interactive {
		Log.Notice("Interactive mode enabled, you will be prompted to handle reapables")
		Events = append(Events, &InteractiveEvent{})
	} else if Conf.Events.Reaper.Enabled {
		Log.Info("Reaper EventReporter enabled.")
		Events = append(Events, events.NewReaperEvent(&Conf.Events.Reaper))
	}

	if Conf.WhitelistTag == "" {
		Log.Warning("WhitelistTag is empty, using 'REAPER_SPARE_ME'")
		Conf.WhitelistTag = "REAPER_SPARE_ME"
	} else {
		Log.Info("Using WhitelistTag '%s'", Conf.WhitelistTag)
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
	// single instance of reaper
	reapRunner := NewReaper(*Conf)

	// Run the reaper process
	reapRunner.Start()

	// run the HTTP server
	api := NewHTTPApi(Conf.HTTP)
	if err := api.Serve(); err != nil {
		Log.Error(err.Error())
	} else {
		// HTTP server successfully started
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
