package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	reaperaws "github.com/mozilla-services/reaper/aws"
	reaperevents "github.com/mozilla-services/reaper/events"
	"github.com/mozilla-services/reaper/http"
	"github.com/mozilla-services/reaper/reaper"
	log "github.com/mozilla-services/reaper/reaperlog"
)

var (
	config         reaper.Config
	eventReporters []reaperevents.EventReporter
)

func init() {
	configFile := flag.String("config", "", "path to config file")
	withoutCloudformationResources := flag.Bool("withoutCloudformationResources", false, "disables dependency checking for Cloudformations (which is slow!)")
	useMozlog := flag.Bool("useMozlog", true, "set to false to disable mozlog output")
	flag.Parse()

	if *useMozlog {
		log.EnableMozlog()
	}

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
				log.Error("Invalid config ", r)
				os.Exit(1)
			}
		}()
		config = *c
		log.Info(fmt.Sprintf("Configuration loaded from %s", *configFile))
	} else {
		// config not successfully loaded -> exit with error
		log.Error("toml", err)
		os.Exit(1)
	}

	// if log file path specified, attempt to load it
	if config.LogFile != "" {
		log.AddLogFile(config.LogFile)
	}

	// if DatadogStatistics EventReporter is enabled
	if config.Events.DatadogStatistics.Enabled {
		log.Info("DatadogStatistics EventReporter enabled.")
		eventReporters = append(eventReporters, reaperevents.NewDatadogStatistics(&config.Events.DatadogStatistics))
	}

	// if DatadogEvents EventReporter is enabled
	if config.Events.DatadogEvents.Enabled {
		log.Info("DatadogEvents EventReporter enabled.")
		eventReporters = append(eventReporters, reaperevents.NewDatadogEvents(&config.Events.DatadogEvents))
	}

	// if Email EventReporter is enabled
	if config.Events.Email.Enabled {
		log.Info("Email EventReporter enabled.")
		eventReporters = append(eventReporters, reaperevents.NewMailer(&config.Events.Email))
		// these methods have pointer receivers
		log.Debug("SMTP Config: %s", &config.Events.Email)
		log.Debug("SMTP From: %s", &config.Events.Email.From)
	}

	// if Tagger EventReporter is enabled
	if config.Events.Tagger.Enabled {
		log.Info("Tagger EventReporter enabled.")
		eventReporters = append(eventReporters, reaperevents.NewTagger(&config.Events.Tagger))
	}

	// if Reaper EventReporter is enabled
	if config.Events.Reaper.Enabled {
		log.Info("Reaper EventReporter enabled.")
		eventReporters = append(eventReporters, reaperevents.NewReaperEvent(&config.Events.Reaper))
	}

	// if WhitelistTag is not set
	if config.WhitelistTag == "" {
		log.Error("WhitelistTag is empty, exiting")
		os.Exit(1)
	} else {
		log.Info("Using WhitelistTag '%s'", config.WhitelistTag)
	}

	if *withoutCloudformationResources {
		config.AWS.WithoutCloudformationResources = true
	}
}

func main() {
	// config and events are vars in the reaper package
	// they NEED to be set before a reaper.Reaper can be initialized
	reaper.SetConfig(&config)
	reaperevents.SetEvents(&eventReporters)

	if config.DryRun {
		log.Info("Dry run mode enabled, no events will be triggered. Enable Extras in Notifications for per-event DryRun notifications.")
		reaperevents.SetDryRun(config.DryRun)
	}

	// Ready() NEEDS to be called after BOTH SetConfig() and SetEvents()
	// it uses those values to set individual EventReporter config values
	// and to init the Reapables map
	reaper.Ready()

	// sets the config variable in Reaper's AWS package
	// this also NEEDS to be set before a Reaper can be started
	reaperaws.SetConfig(&config.AWS)

	// single instance of reaper
	reapRunner := reaper.NewReaper()
	// Run the reaper process
	reapRunner.Start()

	// run the HTTP server
	api := http.NewAPI(config.HTTP)
	if err := api.Serve(); err != nil {
		log.Error(err.Error())
	} else {
		// HTTP server successfully started
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		// waiting for an Interrupt or Kill signal
		// this channel blocks until it receives one
		sig := <-c
		log.Info("Got signal %s, stopping services", sig.String())
		reapRunner.Stop()
	}
}
