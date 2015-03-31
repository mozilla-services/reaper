package main

import (
	"flag"
	"os"

	"github.com/mostlygeek/reaper"
	. "github.com/tj/go-debug"
)

var (
	log    = &reaper.Logger{"EC2"}
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

	stopChan := make(chan struct{})

	reapRunner.Once()

	// start the http server
	// eww.. this seems ugly. the reason is because http.ListenAndServe has
	// no way (unless I write it) to gracefully stop it
	// this is a TODO
	go func(httpSvr *reaper.HTTPApi) {
		err := httpSvr.Serve()
		debug("HTTP Fail: %s", err.Error())
		close(stopChan)
	}(reaper.NewHTTPApi(*Conf))

	// wait in case the http server fails
	<-stopChan
}
