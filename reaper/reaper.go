package reaper

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"

	reaperaws "github.com/milescrabill/reaper/aws"
	reaperevents "github.com/milescrabill/reaper/events"
	"github.com/milescrabill/reaper/filters"
	"github.com/milescrabill/reaper/reapable"
	log "github.com/milescrabill/reaper/reaperlog"
)

var (
	reapables reapable.Reapables
	config    *Config
	events    *[]reaperevents.EventReporter
)

func SetConfig(c *Config) {
	config = c
}
func SetEvents(e *[]reaperevents.EventReporter) {
	events = e
}

// Ready NEEDS to be called for EventReporters and Reapables to be properly initialized
// which means events AND config need to be set BEFORE Ready
func Ready() {
	// set config values for events
	for _, er := range *events {
		er.SetDryRun(config.DryRun)
		er.SetNotificationExtras(config.Notifications.Extras)
	}
}

// Reaper finds resources and deals with them
type Reaper struct {
	stopCh chan struct{}
}

// NewReaper is a Reaper constructor shorthand
func NewReaper() *Reaper {
	return &Reaper{}
}

// Start begins Reaper execution in a new goroutine
func (r *Reaper) Start() {
	if r.stopCh != nil {
		return
	}
	r.stopCh = make(chan struct{})
	go r.start()
}

// Stop closes a Reaper's stop channel
func (r *Reaper) Stop() {
	close(r.stopCh)
}

// unexported start is continuous loop that reaps every
// time interval
func (r *Reaper) start() {
	// make a list of all eligible instances
	for {
		r.Once()
		select {
		case <-time.After(config.Notifications.Interval.Duration):
		case <-r.stopCh: // time to exit!
			log.Debug("Stopping reaper on stop channel message")
			return
		}
	}
}

// Once is run once every time interval by start
// it is intended to handle all reaping logic
func (r *Reaper) Once() {
	r.reap()

	if config.StateFile != "" {
		r.SaveState(config.StateFile)
	}

	log.Notice("Sleeping for %s", config.Notifications.Interval.Duration.String())
}

func (r *Reaper) SaveState(stateFile string) {
	// open file RW, create it if it doesn't exist
	s, err := os.OpenFile(config.StateFile, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0664)
	defer func() { s.Close() }()
	if err != nil {
		log.Error(fmt.Sprintf("Unable to create StateFile '%s'", config.StateFile))
	} else {
		log.Info("States will be saved to %s", config.StateFile)
	}
	// save state to state file
	for r := range reapables.Iter() {
		s.Write([]byte(fmt.Sprintf("%s,%s\n", r.Region, r.ID, r.ReaperState().State.String())))
	}
}

// convenience function that returns a map of instances in ASGs
func allASGInstanceIds(as []reaperaws.AutoScalingGroup) map[string]map[string]bool {
	// maps region to id to bool
	inASG := make(map[string]map[string]bool)
	for _, region := range config.AWS.Regions {
		inASG[region] = make(map[string]bool)
	}
	for _, a := range as {
		for _, instance := range a.Instances {
			// add the instance to the map
			inASG[a.Region][instance] = true
		}
	}
	return inASG
}

// returns ASGs as filterables
func allAutoScalingGroups() []reaperaws.Filterable {
	regions := config.AWS.Regions

	// waitgroup for goroutines
	var wg sync.WaitGroup

	// channel for creating SecurityGroups
	in := make(chan *reaperaws.AutoScalingGroup)

	for _, region := range regions {
		wg.Add(1)

		sum := 0

		// goroutine per region to fetch all security groups
		go func(region string) {
			defer wg.Done()
			api := autoscaling.New(&aws.Config{Region: region})

			// TODO: nextToken paging
			input := &autoscaling.DescribeAutoScalingGroupsInput{}
			resp, err := api.DescribeAutoScalingGroups(input)
			if err != nil {
				// TODO: wee
				log.Error(err.Error())
			}

			for _, a := range resp.AutoScalingGroups {
				sum += 1
				in <- reaperaws.NewAutoScalingGroup(region, a)
			}

			log.Info(fmt.Sprintf("Found %d total AutoScalingGroups in %s", sum, region))
			for _, e := range *events {
				err := e.NewStatistic("reaper.asgs.total", float64(len(in)), []string{fmt.Sprintf("region:%s", region)})
				if err != nil {
					log.Error(fmt.Sprintf("%s", err.Error()))
				}
			}
		}(region)
	}
	// aggregate
	var autoScalingGroups []reaperaws.Filterable
	go func() {
		for a := range in {
			autoScalingGroups = append(autoScalingGroups, a)
		}
	}()

	// synchronous wait for all goroutines in wg to be done
	wg.Wait()

	// done with the channel
	close(in)

	log.Info("Found %d total ASGs.", len(autoScalingGroups))
	return autoScalingGroups
}

func (r *Reaper) reapSnapshots(done chan bool) {
	snapshots := allSnapshots()
	log.Info(fmt.Sprintf("Total snapshots: %d", len(snapshots)))
	done <- true
}

func allSnapshots() []reaperaws.Filterable {
	regions := config.AWS.Regions

	// waitgroup for goroutines
	var wg sync.WaitGroup

	// channel for creating SecurityGroups
	in := make(chan *reaperaws.Snapshot)

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
				in <- reaperaws.NewSnapshot(region, v)
			}

			log.Info(fmt.Sprintf("Found %d total snapshots in %s", sum, region))
			for _, e := range *events {
				e.NewStatistic("reaper.snapshots.total", float64(len(in)), []string{fmt.Sprintf("region:%s", region)})
			}
		}(region)
	}
	// aggregate
	var snapshots []reaperaws.Filterable
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

func (r *Reaper) reapVolumes(done chan bool) {
	volumes := allVolumes()
	log.Info(fmt.Sprintf("Total volumes: %d", len(volumes)))
	for _, e := range *events {
		e.NewStatistic("reaper.volumes.total", float64(len(volumes)), nil)
	}
	done <- true
}

func allVolumes() reaperaws.Volumes {
	regions := config.AWS.Regions

	// waitgroup for goroutines
	var wg sync.WaitGroup

	// channel for creating SecurityGroups
	in := make(chan *reaperaws.Volume)

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
				in <- reaperaws.NewVolume(region, v)
			}

			log.Info(fmt.Sprintf("Found %d total volumes in %s", sum, region))
		}(region)
	}
	// aggregate
	var volumes reaperaws.Volumes
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

func (r *Reaper) reapSecurityGroups(done chan bool) {
	securitygroups := allSecurityGroups()
	log.Info(fmt.Sprintf("Total security groups: %d", len(securitygroups)))
	for _, e := range *events {
		e.NewStatistic("reaper.securitygroups.total", float64(len(securitygroups)), nil)
	}
	done <- true
}

func allSecurityGroups() reaperaws.SecurityGroups {
	regions := config.AWS.Regions

	// waitgroup for goroutines
	var wg sync.WaitGroup

	// channel for creating SecurityGroups
	in := make(chan *reaperaws.SecurityGroup)

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
				in <- reaperaws.NewSecurityGroup(region, sg)
			}

			log.Info(fmt.Sprintf("Found %d total security groups in %s", sum, region))
		}(region)
	}
	// aggregate
	var securityGroups reaperaws.SecurityGroups
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

func (r *Reaper) reap() {
	filterables := allFilterables()
	// TODO: consider slice of pointers
	var asgs []reaperaws.AutoScalingGroup
	for _, f := range filterables {
		switch t := f.(type) {
		case *reaperaws.Instance:
			reapInstance(t)
		case *reaperaws.AutoScalingGroup:
			reapAutoScalingGroup(t)
			asgs = append(asgs, *t)
		case *reaperaws.Snapshot:
			// reapSnapshot(t)
		default:
			log.Error("Reap default case.")
		}
	}

	// TODO: this totally doesn't work because it happens too late
	// basically this doesn't do anything
	// identify instances in an ASG and delete them from Reapables
	instanceIDsInASGs := allASGInstanceIds(asgs)
	for region := range instanceIDsInASGs {
		for instanceID := range instanceIDsInASGs[region] {
			reapables.Delete(region, instanceID)
		}
	}
}

// makes a slice of all filterables by appending
// output of each filterable types aggregator function
func allFilterables() []reaperaws.Filterable {
	var filterables []reaperaws.Filterable
	if config.Instances.Enabled {
		filterables = append(filterables, allInstances()...)
	}
	if config.AutoScalingGroups.Enabled {
		filterables = append(filterables, allAutoScalingGroups()...)
	}
	if config.Snapshots.Enabled {
		filterables = append(filterables, allSnapshots()...)
	}
	return filterables
}

// applies N functions to a filterable F
// returns true if all filters returned true, else returns false
func applyFilters(f reaperaws.Filterable, fs map[string]filters.Filter) bool {
	// recover from potential panics caused by malformed filters
	defer func() {
		if r := recover(); r != nil {
			log.Error(fmt.Sprintf("Recovered in applyFilters with panic: %s", r))
		}
	}()

	// defaults to a match
	matched := true

	// if any of the filters return false -> not a match
	for _, filter := range fs {
		if !f.Filter(filter) {
			matched = false
		}
	}

	// whitelist filter
	if f.Filter(*filters.NewFilter("Tagged", []string{config.WhitelistTag})) {
		// if the filterable matches this filter, then
		// it should be whitelisted, aka not matched
		matched = false
	}

	return matched
}

func reapInstance(i *reaperaws.Instance) {
	filters := config.Instances.Filters
	if applyFilters(i, filters) {
		i.MatchedFilters = fmt.Sprintf(" matched filters %s", reaperaws.PrintFilters(filters))
		if time.Now().After(i.ReaperState().Until) {
			_ = i.IncrementState()
		}
		log.Notice(fmt.Sprintf("Reapable instance discovered: %s.", i.ReapableDescription()))
		for _, e := range *events {
			if err := e.NewStatistic("reaper.instances.reapable", 1, []string{fmt.Sprintf("id:%s", i.ID)}); err != nil {
				log.Error(err.Error())
			}
			if err := e.NewReapableEvent(i); err != nil {
				log.Error(err.Error())
			}
		}

		// add to Reapables if filters matched
		reapables.Put(i.Region, i.ID, i)
	}
}

func reapAutoScalingGroup(a *reaperaws.AutoScalingGroup) {
	filters := config.AutoScalingGroups.Filters
	if applyFilters(a, filters) {
		a.MatchedFilters = fmt.Sprintf(" matched filters %s", reaperaws.PrintFilters(filters))
		if time.Now().After(a.ReaperState().Until) {
			_ = a.IncrementState()
		}
		log.Notice(fmt.Sprintf("Reapable AutoScalingGroup discovered: %s.", a.ReapableDescription()))

		for _, e := range *events {
			if err := e.NewEvent("Reapable AutoScalingGroup discovered", string(a.ReapableEventText().Bytes()), nil, nil); err != nil {
				log.Error(err.Error())
			}
			if err := e.NewStatistic("reaper.autoscalinggroups.reapable", 1, []string{fmt.Sprintf("id:%s", a.ID)}); err != nil {
				log.Error(err.Error())
			}
			if err := e.NewReapableEvent(a); err != nil {
				log.Error(err.Error())
			}
		}
	}

	// add to Reapables
	reapables.Put(a.Region, a.ID, a)
}

func (r *Reaper) terminateUnowned(i *reaperaws.Instance) error {
	log.Info("Terminate UNOWNED instance (%s) %s, owner tag: %s",
		i.ID, i.Name, i.Tag("Owner"))

	if config.DryRun {
		return nil
	}

	// TODO: use success here
	if _, err := i.Terminate(); err != nil {
		log.Error(fmt.Sprintf("Terminate %s error: %s", i.ID, err.Error()))
		return err
	}

	return nil

}

// Terminate by region, id, calls a Reapable's own Terminate method
func Terminate(region, id string) error {
	reapable, err := reapables.Get(region, id)
	if err != nil {
		return err
	}
	_, err = reapable.Terminate()
	if err != nil {
		log.Error("Could not terminate resource with region: %s and id: %s. Error: %s",
			region, id, err.Error())
		return err
	}
	log.Debug("Terminate %s", reapable.ReapableDescriptionShort())

	return nil
}

// ForceStop by region, id, calls a Reapable's own ForceStop method
func ForceStop(region, id string) error {
	reapable, err := reapables.Get(region, id)
	if err != nil {
		return err
	}
	_, err = reapable.ForceStop()
	if err != nil {
		log.Error("Could not stop resource with region: %s and id: %s. Error: %s",
			region, id, err.Error())
		return err
	}
	log.Debug("ForceStop %s", reapable.ReapableDescriptionShort())

	return nil
}

// Stop by region, id, calls a Reapable's own Stop method
func Stop(region, id string) error {
	reapable, err := reapables.Get(region, id)
	if err != nil {
		return err
	}
	_, err = reapable.Stop()
	if err != nil {
		log.Error("Could not stop resource with region: %s and id: %s. Error: %s",
			region, id, err.Error())
		return err
	}
	log.Debug("Stop %s", reapable.ReapableDescriptionShort())

	return nil
}

// allInstances describes every instance in the requested regions
// instances of Instance are created for each *ec2.Instance
// returned as Filterables
func allInstances() []reaperaws.Filterable {

	regions := config.AWS.Regions
	var wg sync.WaitGroup
	in := make(chan *reaperaws.Instance)

	// fetch all info in parallel
	for _, region := range regions {
		// log.Debug("DescribeInstances in %s", region)
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

			for done := false; done != true; {
				input := &ec2.DescribeInstancesInput{
					NextToken: nextToken,
				}
				resp, err := api.DescribeInstances(input)
				if err != nil {
					// probably should do something here...
					log.Debug("EC2 error in %s: %s", region, err.Error())
					return
				}

				for _, r := range resp.Reservations {
					for _, instance := range r.Instances {
						sum += 1
						in <- reaperaws.NewInstance(region, instance)
					}
				}

				if resp.NextToken != nil {
					log.Debug("More results for DescribeInstances in %s", region)
					nextToken = resp.NextToken
				} else {
					done = true
				}
			}

			log.Info("Found %d total instances in %s", sum, region)
			for _, e := range *events {
				go e.NewStatistic("reaper.instances.total", float64(sum), []string{fmt.Sprintf("region:%s", region)})
			}
		}(region)
	}

	var list []reaperaws.Filterable

	// build up the list
	go func() {
		for i := range in {
			list = append(list, i)
		}
	}()

	// wait for all the fetches to finish publishing
	wg.Wait()
	close(in)

	log.Info("Found %d total instances.", len(list))
	return list
}
