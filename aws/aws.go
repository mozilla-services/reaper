package aws

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

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

var (
	config  *AWSConfig
	timeout = time.Tick(100 * time.Millisecond)
)

type AWSConfig struct {
	Notifications    events.NotificationsConfig
	HTTP             events.HTTPConfig
	Regions          []string
	WhitelistTag     string
	DefaultOwner     string
	DefaultEmailHost string
	DryRun           bool

	WithoutCloudformationResources bool
}

func NewAWSConfig() *AWSConfig {
	return &AWSConfig{}
}

func SetAWSConfig(c *AWSConfig) {
	config = c
}

func AllCloudformations() chan *Cloudformation {
	ch := make(chan *Cloudformation)
	// waitgroup for all regions
	wg := sync.WaitGroup{}
	for _, region := range config.Regions {
		wg.Add(1)
		go func(region string) {
			// add region to waitgroup
			api := cloudformation.New(&aws.Config{Region: region})
			err := api.DescribeStacksPages(&cloudformation.DescribeStacksInput{}, func(resp *cloudformation.DescribeStacksOutput, lastPage bool) bool {
				for _, stack := range resp.Stacks {
					ch <- NewCloudformation(region, stack)
				}
				// if we are at the last page, we should not continue
				// the return value of this func is "shouldContinue"
				if lastPage {
					// on the last page, finish this region
					wg.Done()
					return false
				}
				return true
			})
			if err != nil {
				// probably should do something here...
				log.Error(err.Error())
				// don't wait if the API call failed
				wg.Done()
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

func CloudformationResources(c Cloudformation) chan *cloudformation.StackResource {
	ch := make(chan *cloudformation.StackResource)

	if config.WithoutCloudformationResources {
		close(ch)
		return ch
	}

	api := cloudformation.New(&aws.Config{Region: string(c.Region)})
	// TODO: stupid
	stringName := string(c.ID)

	go func() {
		<-timeout

		// this query can fail, so we retry
		didRetry := false
		input := &cloudformation.DescribeStackResourcesInput{StackName: &stringName}

		// initial query
		resp, err := api.DescribeStackResources(input)
		for err != nil {
			sleepTime := 2*time.Second + time.Duration(rand.Intn(2000))*time.Millisecond
			if err != nil {
				// this error is annoying and will come up all the time... so you can disable it
				if strings.Split(err.Error(), ":")[0] == "Throttling" && log.Extras() {
					log.Warning(fmt.Sprintf("StackResources: %s (retrying %s after %ds)", err.Error(), c.ID, sleepTime*1.0/time.Second))
				} else if strings.Split(err.Error(), ":")[0] != "Throttling" {
					// any other errors
					log.Error(fmt.Sprintf("StackResources: %s (retrying %s after %ds)", err.Error(), c.ID, sleepTime*1.0/time.Second))
				}
			}

			// wait a random amount of time... hopefully long enough to beat rate limiting
			time.Sleep(sleepTime)

			// retry query
			resp, err = api.DescribeStackResources(input)
			didRetry = true
		}
		if didRetry && log.Extras() {
			log.Notice("Retry succeeded for %s!", c.ID)
		}
		for _, resource := range resp.StackResources {
			ch <- resource
		}
		close(ch)
	}()
	return ch
}

func ASGInstanceIDs(a *AutoScalingGroup) map[reapable.Region]map[reapable.ID]bool {
	// maps region to id to bool
	inASG := make(map[reapable.Region]map[reapable.ID]bool)
	for _, region := range config.Regions {
		inASG[reapable.Region(region)] = make(map[reapable.ID]bool)
	}
	for _, instanceID := range a.Instances {
		// add the instance to the map
		inASG[a.Region][instanceID] = true
	}
	return inASG
}

// AllAutoScalingGroups describes every AutoScalingGroup in the requested regions
// *AutoScalingGroups are created for every *autoscaling.AutoScalingGroup
// and are passed to a channel
func AllAutoScalingGroups() chan *AutoScalingGroup {
	ch := make(chan *AutoScalingGroup)
	// waitgroup for all regions
	wg := sync.WaitGroup{}
	for _, region := range config.Regions {
		wg.Add(1)
		go func(region string) {
			// add region to waitgroup
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
					return false
				}
				return true
			})
			if err != nil {
				// probably should do something here...
				log.Error(err.Error())
				// don't wait if the API call failed
				wg.Done()
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
		wg.Add(1)
		go func(region string) {
			// add region to waitgroup
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
					return false
				}
				return true
			})
			if err != nil {
				// probably should do something here...
				log.Error(err.Error())
				// don't wait if the API call failed
				wg.Done()
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
		wg.Add(1)
		go func(region string) {
			// add region to waitgroup
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
