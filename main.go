package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/op/go-logging"
)

var (
	Log           = logging.MustGetLogger("Reaper")
	conf          *Config
	mailer        *Mailer
	dryrun        = false
	enableDataDog bool
	events        []EventReporter
)

func init() {
	var configFile string

	flag.StringVar(&configFile, "conf", "", "path to config file")
	flag.BoolVar(&dryrun, "dryrun", false, "dry run, don't make changes")
	flag.BoolVar(&enableDataDog, "datadog", true, "enable DataDog reporting, requires dd-agent running")
	flag.Parse()

	if configFile == "" {
		Log.Error("Config file required", configFile)
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
		Log.Info("Dry run mode enabled, no changes will be made")
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
		Log.Info(fmt.Sprintf("Got signal %s, stopping services", sig.String()))
		Log.Info("Stopping HTTP")
		api.Stop()
		Log.Info("Stopping reaper runner")
		reapRunner.Stop()
	}
}
