package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/mostlygeek/reaper"
	. "github.com/tj/go-debug"
)

var (
	log    = &reaper.Logger{"Reaper"}
	Conf   *reaper.Config
	debug  = Debug("reaper:main")
	Mailer *reaper.Mailer
	DryRun = false
)

func init() {
	var configFile string

	flag.StringVar(&configFile, "conf", "", "path to config file")
	flag.BoolVar(&DryRun, "dryrun", false, "dry run, don't make changes")

	flag.Parse()

	if configFile == "" {
		log.Error("Config file required", configFile)
		os.Exit(1)
	}

	if c, err := reaper.LoadConfig(configFile); err == nil {
		Conf = c
		log.Info("Configuration loaded from", configFile)
		debug("SMTP Config: %s", Conf.SMTP.String())
		debug("SMTP From: %s", Conf.SMTP.From.Address.String())

	} else {
		log.Error("toml", err)
		os.Exit(1)
	}

	Mailer = reaper.NewMailer(*Conf)

	if DryRun {
		log.Info("Dry run mode enabled, no changes will be made")
	}

}

func main() {
	reapRunner := reaper.NewReaper(*Conf, Mailer, log)
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
