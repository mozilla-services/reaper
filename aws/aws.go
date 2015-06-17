package aws

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/milescrabill/reaper/events"
	"github.com/milescrabill/reaper/filters"
	"github.com/milescrabill/reaper/reapable"
	log "github.com/milescrabill/reaper/reaperlog"
)

var config *AWSConfig

type AWSConfig struct {
	Notifications events.NotificationsConfig
	HTTP          events.HTTPConfig
	Regions       []string
	WhitelistTag  string
	DefaultOwner  string
	DryRun        bool
}

func NewAWSConfig() *AWSConfig {
	return &AWSConfig{}
}

func SetAWSConfig(c *AWSConfig) {
	config = c
}

// convenience function that returns a map of instances in ASGs
func AllASGInstanceIds(as []AutoScalingGroup) map[reapable.Region]map[reapable.ID]bool {
	// maps region to id to bool
	inASG := make(map[reapable.Region]map[reapable.ID]bool)
	for _, region := range config.Regions {
		inASG[reapable.Region(region)] = make(map[reapable.ID]bool)
	}
	for _, a := range as {
		for _, instanceID := range a.Instances {
			// add the instance to the map
			inASG[a.Region][instanceID] = true
		}
	}
	return inASG
}

// returns ASGs as filterables
func AllAutoScalingGroups() chan *AutoScalingGroup {
	ch := make(chan *AutoScalingGroup)

	go func(regions []string) {
		for _, region := range regions {
			api := autoscaling.New(&aws.Config{Region: region})

			// TODO: nextToken paging
			input := &autoscaling.DescribeAutoScalingGroupsInput{}
			resp, err := api.DescribeAutoScalingGroups(input)
			if err != nil {
				// TODO: wee
				log.Error(err.Error())
			}

			for _, a := range resp.AutoScalingGroups {
				ch <- NewAutoScalingGroup(region, a)
			}
		}
		close(ch)
	}(config.Regions)

	return ch
}

// allInstances describes every instance in the requested regions
// instances of Instance are created for each *ec2.Instance
// returned as Filterables
func AllInstances() chan *Instance {
	ch := make(chan *Instance)

	go func(regions []string) {
		for _, region := range regions {
			api := ec2.New(&aws.Config{Region: region})

			// repeat until we have everything
			var nextToken *string
			for done := false; done != true; {
				input := &ec2.DescribeInstancesInput{
					NextToken: nextToken,
				}
				resp, err := api.DescribeInstances(input)
				if err != nil {
					// probably should do something here...
					log.Error(err.Error())
				}

				for _, r := range resp.Reservations {
					for _, instance := range r.Instances {
						ch <- NewInstance(region, instance)
					}
				}

				if resp.NextToken != nil {
					log.Debug("More results for DescribeInstances in %s", region)
					nextToken = resp.NextToken
				} else {
					done = true
				}
			}
		}
		close(ch)
	}(config.Regions)

	return ch
}

func AllSnapshots() []filters.Filterable {
	regions := config.Regions

	// waitgroup for goroutines
	var wg sync.WaitGroup

	// channel for creating SecurityGroups
	in := make(chan *Snapshot)

	for _, region := range regions {
		wg.Add(1)

		sum := 0

		// goroutine per region to fetch all security groups
		go func(region string) {
			defer wg.Done()
			api := ec2.New(&aws.Config{Region: region})

			// TODO: nextToken paging
			input := &ec2.DescribeSnapshotsInput{}
			resp, err := api.DescribeSnapshots(input)
			if err != nil {
				// TODO: wee
			}

			for _, v := range resp.Snapshots {
				sum += 1
				in <- NewSnapshot(region, v)
			}
		}(region)
	}
	// aggregate
	var snapshots []filters.Filterable
	go func() {
		for s := range in {
			// Reapables[s.Region][s.ID] = s
			snapshots = append(snapshots, s)
		}
	}()

	// synchronous wait for all goroutines in wg to be done
	wg.Wait()

	// done with the channel
	close(in)

	log.Info("Found %d total snapshots.", len(snapshots))
	return snapshots
}
func AllVolumes() Volumes {
	regions := config.Regions

	// waitgroup for goroutines
	var wg sync.WaitGroup

	// channel for creating SecurityGroups
	in := make(chan *Volume)

	for _, region := range regions {
		wg.Add(1)

		sum := 0

		// goroutine per region to fetch all security groups
		go func(region string) {
			defer wg.Done()
			api := ec2.New(&aws.Config{Region: region})

			// TODO: nextToken paging
			input := &ec2.DescribeVolumesInput{}
			resp, err := api.DescribeVolumes(input)
			if err != nil {
				// TODO: wee
			}

			for _, v := range resp.Volumes {
				sum += 1
				in <- NewVolume(region, v)
			}

			log.Info(fmt.Sprintf("Found %d total volumes in %s", sum, region))
		}(region)
	}
	// aggregate
	var volumes Volumes
	go func() {
		for v := range in {
			// Reapables[v.Region][v.ID] = v
			volumes = append(volumes, v)
		}
	}()

	// synchronous wait for all goroutines in wg to be done
	wg.Wait()

	// done with the channel
	close(in)

	log.Info("Found %d total snapshots.", len(volumes))
	return volumes
}
func AllSecurityGroups() SecurityGroups {
	regions := config.Regions

	// waitgroup for goroutines
	var wg sync.WaitGroup

	// channel for creating SecurityGroups
	in := make(chan *SecurityGroup)

	for _, region := range regions {
		wg.Add(1)

		sum := 0

		// goroutine per region to fetch all security groups
		go func(region string) {
			defer wg.Done()
			api := ec2.New(&aws.Config{Region: region})

			// TODO: nextToken paging
			input := &ec2.DescribeSecurityGroupsInput{}
			resp, err := api.DescribeSecurityGroups(input)
			if err != nil {
				// TODO: wee
			}

			for _, sg := range resp.SecurityGroups {
				sum += 1
				in <- NewSecurityGroup(region, sg)
			}

			log.Info(fmt.Sprintf("Found %d total security groups in %s", sum, region))
		}(region)
	}
	// aggregate
	var securityGroups SecurityGroups
	go func() {
		for sg := range in {
			// Reapables[sg.Region][sg.ID] = sg
			securityGroups = append(securityGroups, sg)
		}
	}()

	// synchronous wait for all goroutines in wg to be done
	wg.Wait()

	// done with the channel
	close(in)

	log.Info("Found %d total security groups.", len(securityGroups))
	return securityGroups
}
