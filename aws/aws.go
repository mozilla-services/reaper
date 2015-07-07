package aws

import (
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/mozilla-services/reaper/events"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
)

const (
	reaperTag           = "REAPER"
	reaperTagSeparator  = "|"
	reaperTagTimeFormat = "2006-01-02 03:04PM MST"
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

func AllCloudformationStacks() chan *CloudformationStack {
	ch := make(chan *CloudformationStack)
	// waitgroup for all regions
	wg := sync.WaitGroup{}
	for _, region := range config.Regions {
		go func(region string) {
			// add region to waitgroup
			wg.Add(1)
			api := cloudformation.New(&aws.Config{Region: region})
			err := api.DescribeStacksPages(&cloudformation.DescribeStacksInput{}, func(resp *cloudformation.DescribeStacksOutput, lastPage bool) bool {
				for _, stack := range resp.Stacks {
					ch <- NewCloudformationStack(region, stack)
				}
				// if we are at the last page, we should not continue
				// the return value of this func is "shouldContinue"
				if lastPage {
					// on the last page, finish this region
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
		// in a separate goroutine, wait for all regions to finish
		// when they finish, close the chan
		wg.Wait()
		close(ch)

	}()
	return ch
}

// AllAutoScalingGroups describes every AutoScalingGroup in the requested regions
// *AutoScalingGroups are created for every *autoscaling.AutoScalingGroup
// and are passed to a channel
func AllAutoScalingGroups() chan *AutoScalingGroup {
	ch := make(chan *AutoScalingGroup)
	// waitgroup for all regions
	wg := sync.WaitGroup{}
	for _, region := range config.Regions {
		go func(region string) {
			// add region to waitgroup
			wg.Add(1)
			api := autoscaling.New(&aws.Config{Region: region})
			err := api.DescribeAutoScalingGroupsPages(&autoscaling.DescribeAutoScalingGroupsInput{}, func(resp *autoscaling.DescribeAutoScalingGroupsOutput, lastPage bool) bool {
				for _, asg := range resp.AutoScalingGroups {
					ch <- NewAutoScalingGroup(region, asg)
				}
				// if we are at the last page, we should not continue
				// the return value of this func is "shouldContinue"
				if lastPage {
					// on the last page, finish this region
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
		// in a separate goroutine, wait for all regions to finish
		// when they finish, close the chan
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
	// waitgroup for all regions
	wg := sync.WaitGroup{}
	for _, region := range config.Regions {
		go func(region string) {
			// add region to waitgroup
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
		// in a separate goroutine, wait for all regions to finish
		// when they finish, close the chan
		wg.Wait()
		close(ch)
	}()
	return ch
}

func AllSecurityGroups() chan *SecurityGroup {
	ch := make(chan *SecurityGroup)
	// waitgroup for all regions
	wg := sync.WaitGroup{}
	for _, region := range config.Regions {
		go func(region string) {
			// add region to waitgroup
			wg.Add(1)
			api := ec2.New(&aws.Config{Region: region})
			resp, err := api.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{})
			for _, sg := range resp.SecurityGroups {
				ch <- NewSecurityGroup(region, sg)
			}
			if err != nil {
				// probably should do something here...
				log.Error(err.Error())
			}
			wg.Done()
		}(region)
	}
	go func() {
		// in a separate goroutine, wait for all regions to finish
		// when they finish, close the chan
		wg.Wait()
		close(ch)

	}()
	return ch
}
