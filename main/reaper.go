package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/mostlygeek/reaper"
	"github.com/op/go-logging"
)

var (
	log           = logging.MustGetLogger("Reaper")
	Conf          *reaper.Config
	Mailer        *reaper.Mailer
	DryRun        = false
	enableDataDog bool
	events        []reaper.EventReporter
)

func init() {
	var configFile string

	flag.StringVar(&configFile, "conf", "", "path to config file")
	flag.BoolVar(&DryRun, "dryrun", false, "dry run, don't make changes")
	flag.BoolVar(&enableDataDog, "datadog", true, "enable DataDog reporting, requires dd-agent running")
	flag.Parse()

	if configFile == "" {
		log.Error("Config file required", configFile)
		os.Exit(1)
	}

	if c, err := reaper.LoadConfig(configFile); err == nil {
		Conf = c
		log.Info("Configuration loaded from %s", configFile)
		log.Debug("SMTP Config: %s", Conf.SMTP.String())
		log.Debug("SMTP From: %s", Conf.SMTP.From.Address.String())

	} else {
		log.Error("toml", err)
		os.Exit(1)
	}

	if enableDataDog {
		log.Info("DataDog enabled.")
		events = append(events, reaper.DataDog{})
	} else {
		events = append(events, reaper.NoEventReporter{})
	}

	Mailer = reaper.NewMailer(*Conf)

	if DryRun {
		log.Info("Dry run mode enabled, no changes will be made")
	}

}

func main() {
	reapRunner := reaper.NewReaper(*Conf, Mailer, log, events)
	if DryRun {
		reapRunner.DryRunOn()
	} else {
		reapRunner.DryRunOff()
	}

	// Run the reaper process
	reapRunner.Start()

	// run the HTTP server
	api := reaper.NewHTTPApi(*Conf)
	if err := api.Serve(); err != nil {
		log.Error(err.Error())
	} else {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		sig := <-c
		log.Info(fmt.Sprintf("Got signal %s, stopping services", sig.String()))
		log.Info("Stopping HTTP")
		api.Stop()
		log.Info("Stopping reaper runner")
		reapRunner.Stop()
	}
}
