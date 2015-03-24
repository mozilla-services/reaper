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

	} else {
		log.Error("toml", err)
		os.Exit(1)
	}

	if DryRun {
		log.Info("Dry run mode enabled, no changes will be made")
	}

}

func main() {

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
			SendNotify1(i)
		case raws.STATE_NOTIFY1:
			SendNotify2(i)
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

func SendNotify1(i *raws.Instance) error {
	Info("Send Notification #1 %s %s => notify1", i.Id(), i.Reaper().State)
	if DryRun {
		return nil
	}
	// send the first notification and update the reaper state
	until := time.Now().Add(Conf.Reaper.SecondNotification.Duration)
	err := i.UpdateReaperState(&raws.State{
		State: raws.STATE_NOTIFY1,
		Until: until,
	})

	if err != nil {
		log.Error(fmt.Sprintf("Send Notification 1 for %s: %s"), i.Id(), err.Error())
		return err
	}

	return nil

}
func SendNotify2(i *raws.Instance) error {
	Info("Send Notification #2 %s %s => notify2", i.Id(), i.Reaper().State)
	if DryRun {
		return nil
	}

	// send the first notification and update the reaper state
	until := time.Now().Add(Conf.Reaper.SecondNotification.Duration)
	err := i.UpdateReaperState(&raws.State{
		State: raws.STATE_NOTIFY2,
		Until: until,
	})

	if err != nil {
		log.Error(fmt.Sprintf("Send Notification 2 for %s: %s"), i.Id(), err.Error())
		return err
	}

	return nil
}

func SendNotification(email, subject, body string) {
}
