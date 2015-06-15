package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	reaperaws "github.com/mostlygeek/reaper/aws"
	reaperevents "github.com/mostlygeek/reaper/events"
	"github.com/mostlygeek/reaper/reaper"
	log "github.com/mostlygeek/reaper/reaperlog"
)

var (
	config *reaper.Config
	events []reaperevents.EventReporter
)

func init() {
	configFile := flag.String("config", "", "path to config file")
	dryRun := flag.Bool("dryrun", true, "dry run, don't trigger events")
	interactive := flag.Bool("interactive", false, "interactive mode, reap based on prompt")
	flag.Parse()

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
				log.Error(fmt.Sprintf("Invalid config, %s", r))
				os.Exit(1)
			}
		}()
		config = c
		log.Info("Configuration loaded from %s", *configFile)
	} else {
		// config not successfully loaded -> exit with error
		log.Error("toml", err)
		os.Exit(1)
	}

	// if log file path specified, attempt to load it
	if config.LogFile != "" {
		log.AddLogFile(config.LogFile)
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
		log.Notice("Dry run mode enabled, no events will be triggered. Enable Extras in Notifications for per-event DryRun notifications.")
		for _, eventReporter := range events {
			eventReporter.SetDryRun(true)
		}
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
