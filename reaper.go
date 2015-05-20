package reaper

import (
	"fmt"
	"sync"
	"time"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/service/ec2"
	"github.com/mostlygeek/reaper/filter"
	"github.com/tj/go-debug"
)

var (
	debugReaper  = debug.Debug("reaper:reaper")
	debugAllInst = debug.Debug("reaper:reaper:allInstances")
)

type Reaper struct {
	conf   Config
	mailer *Mailer
	log    *Logger
	dryrun bool

	stopCh chan struct{}
}

func NewReaper(c Config, m *Mailer, l *Logger) *Reaper {
	return &Reaper{
		conf:   c,
		mailer: m,
		log:    l,
	}
}

func (r *Reaper) DryRunOn()  { r.dryrun = true }
func (r *Reaper) DryRunOff() { r.dryrun = false }

func (r *Reaper) Start() {
	if r.stopCh != nil {
		return
	}
	r.stopCh = make(chan struct{})
	go r.start()
}

func (r *Reaper) Stop() {
	close(r.stopCh)
}

func (r *Reaper) start() {
	// make a list of all eligible instances
	for {
		r.Once()
		debugReaper("Sleeping for %s", r.conf.Reaper.Interval.Duration.String())
		select {
		case <-time.After(r.conf.Reaper.Interval.Duration):
		case <-r.stopCh: // time to exit!
			debugReaper("Stopping reaper on stop channel message")
			return
		}
	}
}

func (r *Reaper) Once() {
	instances := allInstances(r.conf.AWS.Regions)

	r.log.Info(fmt.Sprintf("Total instances: %d", len(instances)))

	// This is where we qualify instances
	filtered := instances.
		Filter(filter.Running).
		Filter(filter.Not(filter.Tagged("REAPER_SPARE_ME"))).
		Filter(filter.ReaperReady(r.conf.Reaper.FirstNotification.Duration)).
		Filter(filter.Tagged("REAP_ME")).
		// can be used to specify a time cutoff
		Filter(filter.LaunchTimeBefore(time.Now().Add(-(time.Second))))

	r.log.Info(fmt.Sprintf("Found %d instances", len(filtered)))

	for _, i := range filtered {
		// determine what to do next based on the last state

		// terminate the instance if we can't determine the owner
		if i.Owner() == nil {
			r.terminateUnowned(i)
			continue
		}

		switch i.Reaper().State {
		case STATE_START, STATE_IGNORE:
			r.sendNotification(i, 1)
		case STATE_NOTIFY1:
			r.sendNotification(i, 2)
		case STATE_NOTIFY2:
			r.terminate(i)
		}
	}
}

func (r *Reaper) info(format string, values ...interface{}) {
	if r.dryrun {
		r.log.Info("(DRYRUN) " + fmt.Sprintf(format, values...))
	} else {
		r.log.Info(fmt.Sprintf(format, values...))
	}
}

func (r *Reaper) terminateUnowned(i *Instance) error {
	r.info("Terminate UNOWNED instance (%s) %s, owner tag: %s",
		i.Id(), i.Name(), i.Tag("Owner"))

	if r.dryrun {
		return nil
	}

	if err := Terminate(i.Region(), i.Id()); err != nil {
		r.log.Error(fmt.Sprintf("Terminate %s error: %s", i.Id(), err.Error()))
		return err
	}

	return nil

}

func (r *Reaper) terminate(i *Instance) error {
	r.info("TERMINATE %s notify2 => terminate", i.Id())
	if err := Terminate(i.Region(), i.Id()); err != nil {
		r.log.Error(fmt.Sprintf("%s failed to terminate error: %s",
			i.Id()), err.Error())
		return err
	}
	return nil
}

func (r *Reaper) stop(i *Instance) error {
	r.info("STOP %s notify2 => stop", i.Id())
	if err := Stop(i.Region(), i.Id()); err != nil {
		r.log.Error(fmt.Sprintf("%s failed to stop error: %s",
			i.Id()), err.Error())
		return err
	}
	return nil
}

func (r *Reaper) sendNotification(i *Instance, notifyNum int) error {
	r.info("Send Notification #%d %s", notifyNum, i.Id())
	if r.dryrun {
		return nil
	}

	var newState StateEnum
	var until time.Time
	switch notifyNum {
	case 2:
		newState = STATE_NOTIFY2
		until = time.Now().Add(r.conf.Reaper.Terminate.Duration)
	default:
		newState = STATE_NOTIFY1
		until = time.Now().Add(r.conf.Reaper.SecondNotification.Duration)
	}

	if err := r.mailer.Notify(notifyNum, i); err != nil {
		debugReaper("Notify %d for %s error %s", notifyNum, i.Id(), err.Error())
		return err
	}

	err := UpdateReaperState(i.Region(), i.Id(), &State{
		State: newState,
		Until: until,
	})

	if err != nil {
		r.log.Error(fmt.Sprintf("Send Notification %d for %s: %s"),
			notifyNum, i.Id(), err.Error())
		return err
	}

	return nil
}

// allInstances describes every instance in the requested regions
func allInstances(regions []string) Instances {

	var wg sync.WaitGroup
	in := make(chan *Instance)

	// fetch all info in parallel
	for _, region := range regions {
		debugAllInst("DescribeInstances in %s", region)
		wg.Add(1)
		go func(region string) {
			defer wg.Done()
			api := ec2.New(&aws.Config{Region: region})

			// repeat until we have everything
			var nextToken *string
			sum := 0
			for done := false; done != true; {
				input := &ec2.DescribeInstancesInput{
					MaxResults: aws.Long(1000),
					NextToken:  nextToken,
					Filters: []*ec2.Filter{
						&ec2.Filter{
							Name:   aws.String("instance-state-name"),
							Values: []*string{aws.String("running")},
						},
					},
				}
				resp, err := api.DescribeInstances(input)
				if err != nil {
					// probably should do something here...
					return
				}

				for _, r := range resp.Reservations {
					for _, instance := range r.Instances {
						sum += 1
						in <- NewInstance(region, instance)
					}
				}

				if resp.NextToken != nil {
					debugAllInst("More results for DescribeInstances in %s", region)
					nextToken = resp.NextToken
				} else {
					done = true
				}
			}

			debugAllInst("Found %d instances in %s", sum, region)

		}(region)
	}

	var list Instances
	done := make(chan struct{})

	// build up the list
	go func() {
		for i := range in {
			list = append(list, i)
		}
		done <- struct{}{}
	}()

	// wait for all the fetches to finish publishing
	wg.Wait()
	close(in)

	// wait for appending goroutine to be done
	<-done

	return list
}
