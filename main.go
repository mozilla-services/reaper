package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/op/go-logging"

	reaperaws "github.com/mostlygeek/reaper/aws"
	reaperevents "github.com/mostlygeek/reaper/events"
	"github.com/mostlygeek/reaper/reaper"
)

// Log, Events, Reapables, and Config are all exported global variables

var (
	// Log -> exported global logger
	log *logging.Logger
	// Config -> exported global config
	config *reaper.Config
	events []reaperevents.EventReporter
)

func init() {
	configFile := flag.String("config", "", "path to config file")
	dryRun := flag.Bool("dryrun", true, "dry run, don't make changes")
	interactive := flag.Bool("interactive", false, "interactive mode, reap based on prompt")
	flag.Parse()

	// set up logging
	log = logging.MustGetLogger("Reaper")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	format := logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} ▶%{color:reset} %{message}")
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)

	// if no config file -> exit with error
	if *configFile == "" {
		log.Error("Config file is a required Argument. Specify with -config='filename'")
		os.Exit(1)
	}

	// if config file path specified, attempt to load it
	if c, err := reaper.LoadConfig(*configFile); err == nil {
		// catches panics loading config
		defer func() {
			if r := recover(); r != nil {
				log.Error("Invalid config, %s", r)
				os.Exit(1)
			}
		}()

		// TODO: extraneous assignment?
		config = c
		log.Info("Configuration loaded from %s", *configFile)
	} else {
		// config not successfully loaded -> exit with error
		log.Error("toml", err)
		os.Exit(1)
	}

	// if log file path specified, attempt to load it
	if config.LogFile != "" {
		// open file write only, append mode
		// create it if it doesn't exist
		f, err := os.OpenFile(config.LogFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
		if err != nil {
			log.Error("Unable to open logfile '%s'", config.LogFile)
		} else {
			// if the file was successfully opened
			log.Info("Logging to %s", config.LogFile)
			// reconfigure logging with stdout and logfile as outputs
			logFileFormat := logging.MustStringFormatter("15:04:05.000: %{shortfunc} ▶ %{level:.4s} ▶ %{message}")
			logFileBackend := logging.NewLogBackend(f, "", 0)
			logFileBackendFormatter := logging.NewBackendFormatter(logFileBackend, logFileFormat)
			logging.SetBackend(backendFormatter, logFileBackendFormatter)
		}
	}

	if config.Events.DataDog.Enabled {
		log.Info("DataDog EventReporter enabled.")
		events = append(events, reaperevents.NewDataDog(&config.Events.DataDog))
	}

	// Events = append(Events, &reaperevents.ErrorEventReporter{})

	if config.Events.Email.Enabled {
		log.Info("Email EventReporter enabled.")
		events = append(events, reaperevents.NewMailer(&config.Events.Email))
		// these methods have pointer receivers
		log.Debug("SMTP Config: %s", &config.Events.Email)
		log.Debug("SMTP From: %s", &config.Events.Email.From)
	}

	if config.Events.Tagger.Enabled {
		log.Info("Tagger EventReporter enabled.")
		events = append(events, reaperevents.NewTagger(&config.Events.Tagger))
	}
	// interactive mode and automatic reaping mode are mutually exclusive
	config.Interactive = *interactive
	if *interactive {
		log.Notice("Interactive mode enabled, you will be prompted to handle reapables. Note: this takes precedence over the Reaper EventReporter.")
		events = append(events, reaperevents.NewInteractiveEvent(&reaperevents.InteractiveEventConfig{Enabled: true}))
	} else if config.Events.Reaper.Enabled {
		log.Info("Reaper EventReporter enabled.")
		events = append(events, reaperevents.NewReaperEvent(&config.Events.Reaper))
	}

	if config.WhitelistTag == "" {
		log.Warning("WhitelistTag is empty, using 'REAPER_SPARE_ME'")
		config.WhitelistTag = "REAPER_SPARE_ME"
	} else {
		log.Info("Using WhitelistTag '%s'", config.WhitelistTag)
	}

	config.DryRun = *dryRun
	if *dryRun {
		log.Notice("Dry run mode enabled, no changes will be made")
	}
}

func main() {
	reaper.SetConfig(config)
	reaper.SetEvents(&events)
	reaper.Ready()

	reaperaws.SetAWSConfig(&config.AWS)

	// single instance of reaper
	reapRunner := reaper.NewReaper()
	// Run the reaper process
	reapRunner.Start()

	// run the HTTP server
	api := reaper.NewHTTPApi(config.HTTP)
	if err := api.Serve(); err != nil {
		log.Error(err.Error())
	} else {
		// HTTP server successfully started
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		sig := <-c
		log.Notice(fmt.Sprintf("Got signal %s, stopping services", sig.String()))
		log.Notice("Stopping HTTP")
		api.Stop()
		log.Notice("Stopping reaper runner")
		reapRunner.Stop()
	}
}
