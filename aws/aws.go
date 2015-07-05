package aws

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/mozilla-services/reaper/events"
	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
)

var config *AWSConfig

type AWSConfig struct {
	Notifications    events.NotificationsConfig
	HTTP             events.HTTPConfig
	Regions          []string
	WhitelistTag     string
	DefaultOwner     string
	DefaultEmailHost string
	DryRun           bool
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

// AllAutoScalingGroups describes every AutoScalingGroup in the requested regions
// *AutoScalingGroups are created for every *autoscaling.AutoScalingGroup
// and are passed to a channel
func AllAutoScalingGroups() chan *AutoScalingGroup {
	ch := make(chan *AutoScalingGroup)
	wg := sync.WaitGroup{}

	for _, region := range config.Regions {
		go func(region string) {
			wg.Add(1)
			api := autoscaling.New(&aws.Config{Region: region})
			err := api.DescribeAutoScalingGroupsPages(&autoscaling.DescribeAutoScalingGroupsInput{}, func(resp *autoscaling.DescribeAutoScalingGroupsOutput, lastPage bool) bool {
				for _, asg := range resp.AutoScalingGroups {
					ch <- NewAutoScalingGroup(region, asg)
				}
				// if we are at the last page, we should not continue
				// the return value of this func is "shouldContinue"
				if lastPage {
					wg.Done()
				}
				return true
			})
			if err != nil {
				// probably should do something here...
				log.Error(err.Error())
			}
		}(region)
	}
	go func() {
		wg.Wait()
		close(ch)

	}()
	return ch
}

// AllInstances describes every instance in the requested regions
// *Instances are created for each *ec2.Instance
// and are passed to a channel
func AllInstances() chan *Instance {
	ch := make(chan *Instance)
	wg := sync.WaitGroup{}

	for _, region := range config.Regions {
		go func(region string) {
			wg.Add(1)
			api := ec2.New(&aws.Config{Region: region})
			// DescribeInstancesPages does autopagination
			err := api.DescribeInstancesPages(&ec2.DescribeInstancesInput{}, func(resp *ec2.DescribeInstancesOutput, lastPage bool) bool {
				for _, res := range resp.Reservations {
					for _, instance := range res.Instances {
						ch <- NewInstance(region, instance)
					}
				}
				// if we are at the last page, we should not continue
				// the return value of this func is "shouldContinue"
				if lastPage {
					wg.Done()
				}
				return true
			})
			if err != nil {
				// probably should do something here...
				log.Error(err.Error())
			}
		}(region)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
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
