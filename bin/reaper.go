package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/mostlygeek/reaper"
	raws "github.com/mostlygeek/reaper/aws"
	"github.com/mostlygeek/reaper/filter"
	. "github.com/tj/go-debug"
)

var (
	log    = &reaper.Logger{"EC2"}
	Conf   *reaper.Config
	debug  = Debug("reaper")
	DryRun = false
	Mailer *reaper.Mailer
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

	// this should:
	// create the web server w/ config
	// create the reaper runner w/ config, w/ smtp service

	// make a list of all eligible instances
	creds := aws.DetectCreds(
		Conf.AWS.AccessID,
		Conf.AWS.AccessSecret,
		Conf.AWS.Token,
	)

	endpoints := raws.NewEndpoints(creds, Conf.AWS.Regions, nil)
	instances := raws.AllInstances(endpoints)

	filtered := instances.
		Filter(filter.Running).
		Filter(filter.Not(filter.Tagged("REAPER_SPARE_ME"))).
		Filter(filter.ReaperReady(Conf.Reaper.FirstNotification.Duration)).
		Filter(filter.Tagged("REAP_ME"))

	log.Info(fmt.Sprintf("Found %d instances", len(filtered)))

	for _, i := range filtered {
		// determine what to do next based on the last state

		// terminate the instance if we can't determine the owner
		if i.Owner() == nil {
			TerminateUnowned(i)
			continue
		}

		switch i.Reaper().State {
		case raws.STATE_START, raws.STATE_IGNORE:
			SendNotify(Mailer, i, 1)
		case raws.STATE_NOTIFY1:
			SendNotify(Mailer, i, 2)
		case raws.STATE_NOTIFY2:
			Terminate(i)
		}
	}
}

func Info(format string, values ...interface{}) {
	if DryRun {
		log.Info("(DRYRUN) " + fmt.Sprintf(format, values...))
	} else {
		log.Info(fmt.Sprintf(format, values...))
	}
}

func TerminateUnowned(i *raws.Instance) error {
	Info("Terminate UNOWNED instance (%s) %s, owner tag: %s",
		i.Id(), i.Name(), i.Tag("Owner"))

	if DryRun {
		return nil
	}

	if err := i.Terminate(); err != nil {
		log.Error(fmt.Sprintf("Terminate %s error: %s", i.Id(), err.Error()))
		return err
	}

	return nil

}

func Terminate(i *raws.Instance) error {
	Info("TERMINATE %s notify2 => terminate", i.Id())
	if err := i.Terminate(); err != nil {
		log.Error(fmt.Sprintf("%s failed to terminate error: %s", i.Id()), err.Error())
		return err
	}
	return nil
}

func SendNotify(mailer *reaper.Mailer, i *raws.Instance, notifyNum int) error {
	Info("Send Notification #%d %s", notifyNum, i.Id())
	if DryRun {
		return nil
	}

	var newState raws.StateEnum
	var until time.Time
	switch notifyNum {
	case 2:
		newState = raws.STATE_NOTIFY2
		until = time.Now().Add(Conf.Reaper.Terminate.Duration)
	default:
		newState = raws.STATE_NOTIFY1
		until = time.Now().Add(Conf.Reaper.SecondNotification.Duration)
	}

	// TODO: Send the notification here..
	if err := mailer.Notify(notifyNum, i); err != nil {
		debug("Notify %d for %s error %s", notifyNum, i.Id(), err.Error())
		return err
	}

	err := i.UpdateReaperState(&raws.State{
		State: newState,
		Until: until,
	})

	if err != nil {
		log.Error(fmt.Sprintf("Send Notification %d for %s: %s"),
			notifyNum, i.Id(), err.Error())
		return err
	}

	return nil
}
