package reaper

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/service/ec2"
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
	events EventReporter

	stopCh chan struct{}
}

func NewReaper(c Config, m *Mailer, l *Logger, e EventReporter) *Reaper {
	return &Reaper{
		conf:   c,
		mailer: m,
		log:    l,
		events: e,
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
	r.reapInstances()
	r.reapSecurityGroups()
	r.reapVolumes()
	r.reapSnapshots()
}

func (r *Reaper) reapSnapshots() {
	volumes := r.allSnapshots(r.conf.AWS)
	r.log.Info(fmt.Sprintf("Total snapshots: %d", len(volumes)))
	r.events.NewStatistic("reaper.snapshots.total", float64(len(volumes)), nil)
}

func (r *Reaper) allSnapshots(conf AWSConfig) Snapshots {
	regions := conf.Regions

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

			// build the filter list
			var filters []*ec2.Filter
			for _, f := range conf.SnapshotFilters {
				filter := &ec2.Filter{Name: aws.String(f.Name)}
				for _, v := range f.Values {
					filter.Values = append(filter.Values, aws.String(v))
				}
				filters = append(filters, filter)
			}

			// TODO: nextToken paging
			input := &ec2.DescribeSnapshotsInput{
				Filters: filters,
			}
			resp, err := api.DescribeSnapshots(input)
			if err != nil {
				// TODO: wee
			}

			for _, v := range resp.Snapshots {
				sum += 1
				in <- NewSnapshot(region, v)
			}

			r.log.Info(fmt.Sprintf("Found %d snapshots in %s", sum, region))
		}(region)
	}
	// aggregate
	var snapshots Snapshots
	go func() {
		for s := range in {
			snapshots = append(snapshots, s)
		}
	}()

	// synchronous wait for all goroutines in wg to be done
	wg.Wait()

	// done with the channel
	close(in)

	return snapshots
}

func (r *Reaper) reapVolumes() {
	volumes := r.allVolumes(r.conf.AWS)
	r.log.Info(fmt.Sprintf("Total volumes: %d", len(volumes)))

	r.events.NewStatistic("reaper.volumes.total", float64(len(volumes)), nil)
}

func (r *Reaper) allVolumes(conf AWSConfig) Volumes {
	regions := conf.Regions

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

			// build the filter list
			var filters []*ec2.Filter
			for _, f := range conf.VolumeFilters {
				filter := &ec2.Filter{Name: aws.String(f.Name)}
				for _, v := range f.Values {
					filter.Values = append(filter.Values, aws.String(v))
				}
				filters = append(filters, filter)
			}

			// TODO: nextToken paging
			input := &ec2.DescribeVolumesInput{
				Filters: filters,
			}
			resp, err := api.DescribeVolumes(input)
			if err != nil {
				// TODO: wee
			}

			for _, v := range resp.Volumes {
				sum += 1
				in <- NewVolume(region, v)
			}

			r.log.Info(fmt.Sprintf("Found %d volumes in %s", sum, region))
		}(region)
	}
	// aggregate
	var volumes Volumes
	go func() {
		for v := range in {
			volumes = append(volumes, v)
		}
	}()

	// synchronous wait for all goroutines in wg to be done
	wg.Wait()

	// done with the channel
	close(in)

	return volumes
}

func (r *Reaper) reapSecurityGroups() {
	securitygroups := r.allSecurityGroups(r.conf.AWS)
	r.log.Info(fmt.Sprintf("Total security groups: %d", len(securitygroups)))
	r.events.NewStatistic("reaper.securitygroups.total", float64(len(securitygroups)), nil)
}

func (r *Reaper) allSecurityGroups(conf AWSConfig) SecurityGroups {
	regions := conf.Regions

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

			// build the filter list
			var filters []*ec2.Filter
			for _, f := range conf.SecurityGroupFilters {
				filter := &ec2.Filter{Name: aws.String(f.Name)}
				for _, v := range f.Values {
					filter.Values = append(filter.Values, aws.String(v))
				}
				filters = append(filters, filter)
			}

			// TODO: nextToken paging
			input := &ec2.DescribeSecurityGroupsInput{
				Filters: filters,
			}
			resp, err := api.DescribeSecurityGroups(input)
			if err != nil {
				// TODO: wee
			}

			for _, sg := range resp.SecurityGroups {
				sum += 1
				in <- NewSecurityGroup(region, sg)
			}

			r.log.Info(fmt.Sprintf("Found %d security groups in %s", sum, region))
		}(region)
	}
	// aggregate
	var securityGroups SecurityGroups
	go func() {
		for sg := range in {
			securityGroups = append(securityGroups, sg)
		}
	}()

	// synchronous wait for all goroutines in wg to be done
	wg.Wait()

	// done with the channel
	close(in)

	return securityGroups
}

func (r *Reaper) reapInstances() {
	instances := allInstances(r.conf.AWS)

	r.log.Info(fmt.Sprintf("Total instances: %d", len(instances)))
	r.events.NewStatistic("reaper.instances.total", float64(len(instances)), nil)

	// This is where we qualify instances
	// filtered := instances.
	// 	Filter(filter.Not(filter.Tagged("REAPER_SPARE_ME"))).
	// 	// TODO: line below must be changed before actually running
	// 	// Filter(filter.ReaperReady(r.conf.Reaper.FirstNotification.Duration)).
	// 	Filter(filter.Tagged("REAP_ME")).
	// 	// can be used to specify a time cutoff
	// 	Filter(filter.LaunchTimeBeforeOrEqual(time.Now().Add(-(time.Second))))

	filtered := instances.Tagged("REAP_ME")

	r.log.Info(fmt.Sprintf("Found %d reapable instances", len(filtered)))
	r.events.NewStatistic("reaper.instances.reapable", float64(len(filtered)), nil)

	for _, i := range filtered {
		r.events.NewReapableInstanceEvent(i)

		if i.Owned() {
			r.log.Info(fmt.Sprintf("Reapable: instance %s owned by %s", i.Id(), i.Owner()))
		}

		// terminate the instance if we can't determine the owner
		if i.Owner() == nil {
			r.terminateUnowned(i)

			title := "Reaper terminated unowned instance"
			text := fmt.Sprintf("Unowned instance %s was terminated.", i.Id())

			r.events.NewEvent(title, text, nil, nil)

			continue
		}

		switch i.Reaper().State {
		case STATE_START, STATE_IGNORE:
			r.sendNotification(i, 1)
			r.events.NewEvent("Reaper sent notification 1", fmt.Sprintf("Notification 1 sent to %s for instance %s.", i.Owner(), i.Id()), nil, nil)

		case STATE_NOTIFY1:
			r.sendNotification(i, 2)
			r.events.NewEvent("Reaper sent notification 2", fmt.Sprintf("Notification 2 sent to %s for instance %s.", i.Owner(), i.Id()), nil, nil)

		case STATE_NOTIFY2:
			r.terminate(i)
			r.events.NewEvent("Reaper terminated instance", fmt.Sprintf("Instance owned by %s with id: %s was terminated.", i.Owner(), i.Id()), nil, nil)
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

	if r.dryrun {
		return nil
	}

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
func allInstances(conf AWSConfig) Instances {

	regions := conf.Regions
	var wg sync.WaitGroup
	in := make(chan *Instance)

	// fetch all info in parallel
	for _, region := range regions {
		debugAllInst("DescribeInstances in %s", region)
		wg.Add(1)

		go func(region string) {
			defer wg.Done()
			api := ec2.New(&aws.Config{Region: region})

			/* //uncomment to enable a whole bunch of debug output
			api.Config.LogLevel = 1
			api.AddDebugHandlers()
			*/

			// repeat until we have everything
			var nextToken *string
			sum := 0

			// build the filter list
			var filters []*ec2.Filter
			for _, f := range conf.InstanceFilters {
				filter := &ec2.Filter{Name: aws.String(f.Name)}
				for _, v := range f.Values {
					filter.Values = append(filter.Values, aws.String(v))
				}
				filters = append(filters, filter)
			}

			for _, f := range filters {
				vals := make([]string, len(f.Values), len(f.Values))
				for i, v := range f.Values {
					vals[i] = *v
				}
				debugAllInst(" > filter %s: %s", *f.Name, strings.Join(vals, ", "))
			}
			_ = filters

			for done := false; done != true; {
				input := &ec2.DescribeInstancesInput{
					NextToken: nextToken,
					Filters:   filters,
				}
				resp, err := api.DescribeInstances(input)
				if err != nil {
					// probably should do something here...
					debugAllInst("EC2 error in %s: %s", region, err.Error())
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

	// build up the list
	go func() {
		for i := range in {
			list = append(list, i)
		}
	}()

	// wait for all the fetches to finish publishing
	wg.Wait()
	close(in)

	return list
}
