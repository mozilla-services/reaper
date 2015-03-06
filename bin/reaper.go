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

	filtered := all.Filter(filter.Id("i-25004b29"))

	for _, i := range filtered {
		fmt.Println(i.Region(), i.Id(), i.Name(), i.Owner(), i.State())

		if i.State().State != reaper.STATE_IGNORE {
			fmt.Println("delaying ", i.Id())
			err := i.Delay(time.Now().Add(time.Hour * 48))
			if err != nil {
				fmt.Println("ERROR", err)
			}
		}

	}

}
