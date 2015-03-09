package main

import (
	"fmt"
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/ec2"
	"github.com/mostlygeek/reaper"
	"github.com/mostlygeek/reaper/filter"
	"time"
)

var (
	_ = fmt.Println
)

func main() {
	creds := aws.DetectCreds("", "", "")
	endpoints := reaper.EndpointMap{"us-west-2": ec2.New(creds, "us-west-2", nil)}

	/*
		endpoints, err := reaper.AllEndpoints(creds)
		if err != nil {
			panic(err)
		}
	*/

	all := reaper.AllInstances(endpoints)

	/*
		filtered := all.
			Filter(filter.NotOwned).
			Filter(filter.NotAutoscaled)
	*/

	filtered := all.
		Filter(filter.Tagged("REAP_ME")).
		Filter(filter.Running)

	for _, i := range filtered {
		reaper.Log.Info(i.Region(), i.Id(), i.State(), i.Name(), i.Owner(), i.Reaper())

		if i.Reaper().State != reaper.STATE_IGNORE {
			reaper.Log.Info("Setting ignore: ", i.Id())
			err := i.Ignore(time.Now().Add(time.Hour * 2))
			if err != nil {
				reaper.Log.Err(err)
			}
		}

	}

}
