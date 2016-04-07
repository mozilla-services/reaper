package aws

import (
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
	scalerTag           = "REAPER_AUTOSCALER"

	// default schedule options
	scaleDownPacificBusinessHours = "0 30 1 * * 2-6"  // 1:30 UTC Tuesday-Saturday is 18:30 Pacific Monday-Friday
	scaleUpPacificBusinessHours   = "0 30 14 * * 1-5" // 14:30 UTC Monday-Friday is 7:30 Pacific Monday-Friday
	scaleDownEasternBusinessHours = "0 30 22 * * 1-5" // 22:30 UTC Monday-Friday is 18:30 Eastern Monday-Friday
	scaleUpEasternBusinessHours   = "0 30 11 * * 1-5" // 11:30 UTC Monday-Friday is 7:30 Eastern Monday-Friday
	scaleDownCESTBusinessHours    = "0 30 16 * * 2-6" // 16:30 UTC Tuesday-Saturday is 18:30 CEST Monday-Friday
	scaleUpCESTBusinessHours      = "0 30 5 * * 1-5"  // 5:30 UTC Monday-Friday is 7:30 CEST Monday-Friday
)

var (
	// package wide global
	config  *Config
	timeout = time.Tick(100 * time.Millisecond)
)

// Scaler is an interface used to configure scheduable resources' schedules
type Scaler interface {
	SetScaleDownString(s string)
	SetScaleUpString(s string)
	SaveSchedule()
}

// Config stores configuration for the aws package
type Config struct {
	Notifications    events.NotificationsConfig
	HTTP             events.HTTPConfig
	Regions          []string
	WhitelistTag     string
	DefaultOwner     string
	DefaultEmailHost string
	DryRun           bool

	WithoutCloudformationResources bool
}

// NewConfig returns a new Config for the aws package
func NewConfig() *Config {
	return &Config{}
}

// SetConfig sets the Config for the aws package
// package wide global
func SetConfig(c *Config) {
	config = c
}

// AllCloudformations returns a chan of Cloudformations, sourced from the AWS API
func AllCloudformations() chan *Cloudformation {
	ch := make(chan *Cloudformation, len(config.Regions))
	// waitgroup for all regions
	wg := sync.WaitGroup{}
	for _, region := range config.Regions {
		wg.Add(1)
		go func(region string) {
			defer wg.Done()
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
					return false
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

// cloudformationResources returns a chan of CloudformationResources, sourced from the AWS API
// there is rate limiting in the AWS API for CloudformationResources, so we delay
// this is skippable with the CLI flag -withoutCloudformationResources
func cloudformationResources(region, id string) chan *cloudformation.StackResource {
	ch := make(chan *cloudformation.StackResource)

	if config.WithoutCloudformationResources {
		close(ch)
		return ch
	}

	api := cloudformation.New(&aws.Config{Region: region})
	go func() {
		<-timeout

		// this query can fail, so we retry
		didRetry := false
		input := &cloudformation.DescribeStackResourcesInput{StackName: &id}

		// initial query
		resp, err := api.DescribeStackResources(input)
		for err != nil {
			sleepTime := 2*time.Second + time.Duration(rand.Intn(2000))*time.Millisecond
			if err != nil {
				// this error is annoying and will come up all the time... so you can disable it
				if strings.Split(err.Error(), ":")[0] == "Throttling" && log.Extras() {
					log.Warning("StackResources: %s (retrying %s after %ds)", err.Error(), id, sleepTime*1.0/time.Second)
				} else if strings.Split(err.Error(), ":")[0] != "Throttling" {
					// any other errors
					log.Error("StackResources: %s (retrying %s after %ds)", err.Error(), id, sleepTime*1.0/time.Second)
				}
			}

			// wait a random amount of time... hopefully long enough to beat rate limiting
			time.Sleep(sleepTime)

			// retry query
			resp, err = api.DescribeStackResources(input)
			didRetry = true
		}
		if didRetry && log.Extras() {
			log.Notice("Retry succeeded for %s!", id)
		}
		for _, resource := range resp.StackResources {
			ch <- resource
		}
		close(ch)
	}()
	return ch
}

// AutoScalingGroupInstanceIDs returns a map of regions to a map of ids to bools
// the bool value is whether the instance with that region/id is in an ASG
func AutoScalingGroupInstanceIDs(a *AutoScalingGroup) map[reapable.Region]map[reapable.ID]bool {
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
	ch := make(chan *AutoScalingGroup, len(config.Regions))
	// waitgroup for all regions
	wg := sync.WaitGroup{}
	for _, region := range config.Regions {
		wg.Add(1)
		go func(region string) {
			defer wg.Done()
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
					return false
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
	ch := make(chan *Instance, len(config.Regions))
	// waitgroup for all regions
	wg := sync.WaitGroup{}
	for _, region := range config.Regions {
		wg.Add(1)
		go func(region string) {
			defer wg.Done()
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
					return false
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

// AllVolumes describes every instance in the requested regions
// *Volumes are created for each *ec2.Volume
// and are passed to a channel
func AllVolumes() chan *Volume {
	ch := make(chan *Volume, len(config.Regions))
	// waitgroup for all regions
	wg := sync.WaitGroup{}
	for _, region := range config.Regions {
		wg.Add(1)
		go func(region string) {
			defer wg.Done()
			// add region to waitgroup
			api := ec2.New(&aws.Config{Region: region})
			// DescribeVolumesPages does autopagination
			err := api.DescribeVolumesPages(&ec2.DescribeVolumesInput{}, func(resp *ec2.DescribeVolumesOutput, lastPage bool) bool {
				for _, vol := range resp.Volumes {
					ch <- NewVolume(region, vol)
				}
				// if we are at the last page, we should not continue
				// the return value of this func is "shouldContinue"
				if lastPage {
					return false
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

// AllSecurityGroups describes every instance in the requested regions
// *SecurityGroups are created for each *ec2.SecurityGroup
// and are passed to a channel
func AllSecurityGroups() chan *SecurityGroup {
	ch := make(chan *SecurityGroup, len(config.Regions))
	// waitgroup for all regions
	wg := sync.WaitGroup{}
	for _, region := range config.Regions {
		wg.Add(1)
		go func(region string) {
			defer wg.Done()
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
