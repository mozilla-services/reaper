package reaper

import (
	"fmt"
	"os"
	"time"

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

	if r := reapable.NewReapables(config.AWS.Regions); r != nil {
		reapables = *r
	} else {
		log.Error("reapables improperly initialized")
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

func (r *Reaper) reap() {
	owned, unowned := allReapables()
	var filtered []reaperevents.Reapable

	// TODO: consider slice of pointers
	var asgs []reaperaws.AutoScalingGroup

	// apply filters and trigger events for owned resources
	// for each owner in the owner map
	for _, ownerMap := range owned {
		// apply filters to their resources
		resources := applyFilters(ownerMap)
		filtered = append(filtered, resources...)
		// trigger a per owner batch event
		for _, e := range *events {
			if err := e.NewBatchReapableEvent(resources); err != nil {
				log.Error(err.Error())
			}
		}
	}

	// apply filters and trigger events for unowned resources
	filteredUnowned := applyFilters(unowned)
	filtered = append(filtered, filteredUnowned...)
	for _, r := range filteredUnowned {
		for _, e := range *events {
			if err := e.NewReapableEvent(r); err != nil {
				log.Error(err.Error())
			}
		}
	}

	// filtered has _all_ resources post filtering
	for _, f := range filtered {
		switch t := f.(type) {
		case *reaperaws.Instance:
			reapInstance(t)
		case *reaperaws.AutoScalingGroup:
			reapAutoScalingGroup(t)
			asgs = append(asgs, *t)
		default:
			log.Error("Reap default case.")
		}
	}

	// TODO: this totally doesn't work because it happens too late
	// basically this doesn't do anything
	// identify instances in an ASG and delete them from Reapables
	instanceIDsInASGs := reaperaws.AllASGInstanceIds(asgs)
	for region := range instanceIDsInASGs {
		for instanceID := range instanceIDsInASGs[region] {
			reapables.Delete(region, instanceID)
		}
	}
}

func getInstances() chan *reaperaws.Instance {
	ch := make(chan *reaperaws.Instance)
	go func() {
		instanceCh := reaperaws.AllInstances()
		regionSums := make(map[reapable.Region]int)
		sum := 0
		for instance := range instanceCh {
			regionSums[instance.Region]++
			sum++
			ch <- instance
		}
		for region, regionSum := range regionSums {
			log.Info(fmt.Sprintf("Found %d total Instances in %s", regionSum, region))
			for _, e := range *events {
				err := e.NewStatistic("reaper.instances.total", float64(sum), []string{fmt.Sprintf("region:%s", region)})
				if err != nil {
					log.Error(fmt.Sprintf("%s", err.Error()))
				}
			}
		}
		close(ch)
	}()
	return ch
}

func getAutoScalingGroups() chan *reaperaws.AutoScalingGroup {
	ch := make(chan *reaperaws.AutoScalingGroup)
	go func() {
		asgCh := reaperaws.AllAutoScalingGroups()
		regionSums := make(map[reapable.Region]int)
		sum := 0
		for asg := range asgCh {
			regionSums[asg.Region]++
			sum++
			ch <- asg
		}
		for region, regionSum := range regionSums {
			log.Info(fmt.Sprintf("Found %d total AutoScalingGroups in %s", regionSum, region))
			for _, e := range *events {
				err := e.NewStatistic("reaper.asgs.total", float64(sum), []string{fmt.Sprintf("region:%s", region)})
				if err != nil {
					log.Error(fmt.Sprintf("%s", err.Error()))
				}
			}
		}
		close(ch)
	}()
	return ch
}

// makes a slice of all filterables by appending
// output of each filterable types aggregator function
func allReapables() (map[string][]reaperevents.Reapable, []reaperevents.Reapable) {
	owned := make(map[string][]reaperevents.Reapable)
	var unowned []reaperevents.Reapable
	if config.Instances.Enabled {
		// get all instances
		for i := range getInstances() {
			// group instances by owner
			if i.Owner() != nil {
				owned[i.Owner().Name] = append(owned[i.Owner().Name], i)
			} else {
				// if unowned, append to filterables
				unowned = append(unowned, i)
			}
		}
		// all instances are appended to instances or filterables
	}
	if config.AutoScalingGroups.Enabled {
		for a := range getAutoScalingGroups() {
			// group instances by owner
			if a.Owner() != nil {
				owned[a.Owner().Name] = append(owned[a.Owner().Name], a)
			} else {
				// if unowned, append to filterables
				unowned = append(unowned, a)
			}
		}
	}
	if config.Snapshots.Enabled {

	}
	return owned, unowned
}

// takes an array of filterables
// actually (reaperevents.Reapables because I suck at the type system)
// and spits out a filtered array BY THE INDIVIDUAL
func applyFilters(filterables []reaperevents.Reapable) []reaperevents.Reapable {
	// recover from potential panics caused by malformed filters
	defer func() {
		if r := recover(); r != nil {
			log.Error(fmt.Sprintf("Recovered in applyFilters with panic: %s", r))
		}
	}()

	var gs []reaperevents.Reapable
	for _, filterable := range filterables {
		fs := make(map[string]filters.Filter)
		switch t := filterable.(type) {
		case *reaperaws.Instance:
			fs = config.Instances.Filters
			t.MatchedFilters = fmt.Sprintf(" matched filters %s", filters.PrintFilters(fs))
		case *reaperaws.AutoScalingGroup:
			fs = config.AutoScalingGroups.Filters
			t.MatchedFilters = fmt.Sprintf(" matched filters %s", filters.PrintFilters(fs))
		default:
			log.Warning("You probably screwed and need to make sure applyFilters works!")
			return []reaperevents.Reapable{}
		}

		// defaults to a match
		matched := true

		// if any of the filters return false -> not a match
		for _, filter := range fs {
			if !filterable.Filter(filter) {
				matched = false
			}
		}

		// whitelist filter
		if filterable.Filter(*filters.NewFilter("Tagged", []string{config.WhitelistTag})) {
			// if the filterable matches this filter, then
			// it should be whitelisted, aka not matched
			matched = false
		}

		if matched {
			gs = append(gs, filterable)
		}
	}
	return gs
}

func reapInstance(i *reaperaws.Instance) {
	if time.Now().After(i.ReaperState().Until) {
		_ = i.IncrementState()
	}
	log.Notice(fmt.Sprintf("Reapable Instance discovered: %s.", i.ReapableDescription()))

	// add to Reapables if filters matched
	reapables.Put(i.Region, i.ID, i)
}

func reapAutoScalingGroup(a *reaperaws.AutoScalingGroup) {
	if time.Now().After(a.ReaperState().Until) {
		_ = a.IncrementState()
	}
	log.Notice(fmt.Sprintf("Reapable AutoScalingGroup discovered: %s.", a.ReapableDescription()))

	// add to Reapables if filters matched
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
func Terminate(region reapable.Region, id reapable.ID) error {
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
func ForceStop(region reapable.Region, id reapable.ID) error {
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
func Stop(region reapable.Region, id reapable.ID) error {
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
