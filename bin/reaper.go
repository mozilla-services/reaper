package main

import (
	"fmt"
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/ec2"
	"github.com/mostlygeek/reaper"
	"github.com/mostlygeek/reaper/filter"
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
	fmt.Println(len(all), cap(all))

	filtered := all.
		Filter(filter.NotOwned).
		Filter(filter.NotAutoscaled)

	for _, i := range filtered {
		fmt.Println(i.Region(), i.Id(), i.Name(), i.Owner(), i.State())
	}

}
