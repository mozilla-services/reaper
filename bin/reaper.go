package main

import (
	"flag"
	"fmt"
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/ec2"
	"github.com/mostlygeek/reaper"
	"github.com/mostlygeek/reaper/filter"
	. "github.com/tj/go-debug"
	"os"
	"time"
)

var (
	_     = fmt.Println
	log   = &reaper.Logger{"EC2"}
	Conf  *reaper.Config
	debug = Debug("reaper:EC2")
)

func init() {
	var configFile string

	flag.StringVar(&configFile, "conf", "", "path to config file")
	flag.Parse()

	if configFile == "" {
		log.Err("Config file required", configFile)
		os.Exit(1)
	} else {
	}

	if c, err := reaper.LoadConfig(configFile); err == nil {
		Conf = c
		log.Info("Loaded configuration", configFile)
	} else {
		log.Err("toml", err)
		os.Exit(1)
	}

}

func main() {

	creds := aws.DetectCreds(
		Conf.Credentials.AccessID,
		Conf.Credentials.AccessSecret,
		Conf.Credentials.Token,
	)

	endpoints := reaper.EndpointMap{"us-west-2": ec2.New(creds, "us-west-2", nil)}

	debug("Fetching All Instances")
	all := reaper.AllInstances(endpoints)

	filtered := all.
		Filter(filter.Tagged("REAP_ME")).
		Filter(filter.Running)

	debug("Found %d instances, filtered to %d", len(all), len(filtered))

	for _, i := range filtered {
		log.Info(i.Region(), i.Id(), i.Name(), i.Owner(), i.Reaper())

		if i.Reaper().State != reaper.STATE_IGNORE {
			log.Info("Setting ignore: ", i.Id())
			err := i.Ignore(time.Now().Add(time.Hour * 2))
			if err != nil {
				log.Err(err)
			}
		}

	}

}
