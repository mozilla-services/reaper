package reaper

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	reaperaws "github.com/mozilla-services/reaper/aws"
	reaperevents "github.com/mozilla-services/reaper/events"
	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
)

var (
	reapables   reapable.Reapables
	savedstates map[reapable.Region]map[reapable.ID]*state.State
	config      *Config
	events      *[]reaperevents.EventReporter
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
	}

	if r := reapable.NewReapables(config.AWS.Regions); r != nil {
		reapables = *r
	} else {
		log.Error("reapables improperly initialized")
	}
}

// Reaper finds resources and deals with them
type Reaper struct {
	stopCh chan bool
}

// NewReaper is a Reaper constructor shorthand
func NewReaper() *Reaper {
	return &Reaper{
		stopCh: make(chan bool),
	}
}

// Start begins Reaper execution in a new goroutine
func (r *Reaper) Start() {
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
	// if loading from saved state file (overriding AWS states)
	if config.LoadFromStateFile {
		r.LoadState(config.StateFile)
	}

	r.reap()

	if config.StateFile != "" {
		r.SaveState(config.StateFile)
	}

	log.Notice("Sleeping for %s", config.Notifications.Interval.Duration.String())
}

func (r *Reaper) SaveState(stateFile string) {
	// open file RW, create it if it doesn't exist
	s, err := os.OpenFile(stateFile, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0664)
	defer func() { s.Close() }()
	// save state to state file
	for r := range reapables.Iter() {
		s.Write([]byte(fmt.Sprintf("%s,%s,%s\n", r.Region, r.ID, r.ReaperState().String())))
	}
	if err != nil {
		log.Error(fmt.Sprintf("Unable to create StateFile '%s'", stateFile))
	} else {
		log.Info("States saved to %s", stateFile)
	}
}

func (r *Reaper) LoadState(stateFile string) {
	// open file RDONLY
	s, err := os.OpenFile(stateFile, os.O_RDONLY, 0664)
	defer func() { s.Close() }()

	// init saved state map
	savedstates = make(map[reapable.Region]map[reapable.ID]*state.State)
	for _, region := range config.AWS.Regions {
		savedstates[reapable.Region(region)] = make(map[reapable.ID]*state.State)
	}

	// load state from state file
	scanner := bufio.NewScanner(s)
	for scanner.Scan() {
		line := strings.Split(scanner.Text(), ",")
		// there should be 3 sections in a saved state line
		if len(line) != 3 {
			log.Error(fmt.Sprintf("Malformed saved state %s", scanner.Text()))
			continue
		}
		region := reapable.Region(line[0])
		id := reapable.ID(line[1])
		savedState := state.NewStateWithTag(line[2])

		savedstates[region][id] = savedState
	}
	if err != nil {
		log.Error(fmt.Sprintf("Unable to open StateFile '%s'", stateFile))
	} else {
		log.Info("States loaded from %s", stateFile)
	}
}

func (r *Reaper) reap() {
	owned, unowned := allReapables()
	var filtered []reaperevents.Reapable

	// TODO: consider slice of pointers
	var asgs []reaperaws.AutoScalingGroup

	// filtered, owned resources
	filteredOwned := make(map[string][]reaperevents.Reapable)

	// apply filters and trigger events for owned resources
	// for each owner in the owner map
	for owner, ownerMap := range owned {
		// apply filters to their resources
		resources := applyFilters(ownerMap)
		// if there's only one resource for this owner
		if len(resources) == 1 {
			// no point sending a batch
			// so instead just add it to unowned
			// to be individually sent
			unowned = append(unowned, resources...)
			continue
		}

		// append the resources to filtered
		// so that reap methods are called on them
		filtered = append(filtered, resources...)

		// add resources (post filter) to filteredOwned for batch events
		filteredOwned[owner] = resources
	}

	// apply filters and trigger events for unowned resources
	filteredUnowned := applyFilters(unowned)
	filtered = append(filtered, filteredUnowned...)

	filteredInstanceSums := make(map[reapable.Region]int)
	filteredASGSums := make(map[reapable.Region]int)
	filteredCloudformationStackSums := make(map[reapable.Region]int)

	// filtered has _all_ resources post filtering
	for _, f := range filtered {
		switch t := f.(type) {
		case *reaperaws.Instance:
			filteredInstanceSums[t.Region]++
			reapInstance(t)
		case *reaperaws.AutoScalingGroup:
			filteredASGSums[t.Region]++
			reapAutoScalingGroup(t)
			asgs = append(asgs, *t)
		case *reaperaws.CloudformationStack:
			filteredCloudformationStackSums[t.Region]++
			reapCloudformationStack(t)
		default:
			log.Error("Reap default case.")
		}
	}

	// trigger batch events for each filtered owned resource in a goroutine
	// for each owner in the owner map
	for _, ownerMap := range filteredOwned {
		// trigger a per owner batch event
		for _, e := range *events {
			if err := e.NewBatchReapableEvent(ownerMap); err != nil {
				log.Error(err.Error())
			}
		}
	}

	// trigger events for each filtered unowned resource in a goroutine
	for _, r := range filteredUnowned {
		for _, e := range *events {
			if err := e.NewReapableEvent(r); err != nil {
				log.Error(err.Error())
			}
		}
	}

	// post statistics
	for _, e := range *events {
		for region, sum := range filteredInstanceSums {
			err := e.NewStatistic("reaper.instances.filtered", float64(sum), []string{fmt.Sprintf("region:%s", region)})
			if err != nil {
				log.Error(fmt.Sprintf("%s", err.Error()))
			}
		}
		for region, sum := range filteredASGSums {
			err := e.NewStatistic("reaper.asgs.filtered", float64(sum), []string{fmt.Sprintf("region:%s", region)})
			if err != nil {
				log.Error(fmt.Sprintf("%s", err.Error()))
			}
		}
		for region, sum := range filteredCloudformationStackSums {
			err := e.NewStatistic("reaper.cloudformations.filtered", float64(sum), []string{fmt.Sprintf("region:%s", region)})
			if err != nil {
				log.Error(fmt.Sprintf("%s", err.Error()))
			}
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
		instanceTypeSums := make(map[reapable.Region]map[string]int)
		for instance := range instanceCh {
			// restore saved state from file
			savedstate, ok := savedstates[instance.Region][instance.ID]
			if ok {
				instance.SetReaperState(savedstate)
			}

			// make the map if it is not initialized
			if instanceTypeSums[instance.Region] == nil {
				instanceTypeSums[instance.Region] = make(map[string]int)
			}
			instanceTypeSums[instance.Region][instance.InstanceType]++

			regionSums[instance.Region]++
			ch <- instance
		}

		for region, sum := range regionSums {
			log.Info(fmt.Sprintf("Found %d total Instances in %s", sum, region))
		}

		for _, e := range *events {
			for region, regionMap := range instanceTypeSums {
				for instanceType, instanceTypeSum := range regionMap {
					err := e.NewStatistic("reaper.instances.instancetype", float64(instanceTypeSum), []string{fmt.Sprintf("region:%s,instancetype:%s", region, instanceType)})
					if err != nil {
						log.Error(fmt.Sprintf("%s", err.Error()))
					}
				}
			}

			for region, regionSum := range regionSums {
				err := e.NewStatistic("reaper.instances.total", float64(regionSum), []string{fmt.Sprintf("region:%s", region)})
				if err != nil {
					log.Error(fmt.Sprintf("%s", err.Error()))
				}
			}
		}
		close(ch)
	}()
	return ch
}

func getCloudformationStacks() chan *reaperaws.CloudformationStack {
	ch := make(chan *reaperaws.CloudformationStack)
	go func() {
		cfs := reaperaws.AllCloudformationStacks()
		regionSums := make(map[reapable.Region]int)
		for cf := range cfs {
			// restore saved state from file
			savedstate, ok := savedstates[cf.Region][cf.ID]
			if ok {
				cf.SetReaperState(savedstate)
			}

			regionSums[cf.Region]++
			ch <- cf
		}
		for region, sum := range regionSums {
			log.Info(fmt.Sprintf("Found %d total Cloudformation Stacks in %s", sum, region))
		}
		for _, e := range *events {
			for region, regionSum := range regionSums {
				err := e.NewStatistic("reaper.cloudformations.total", float64(regionSum), []string{fmt.Sprintf("region:%s", region)})
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
		asgSizeSums := make(map[reapable.Region]map[int64]int)
		for asg := range asgCh {
			// restore saved state from file
			savedstate, ok := savedstates[asg.Region][asg.ID]
			if ok {
				asg.SetReaperState(savedstate)
			}

			// make the map if it is not initialized
			if asgSizeSums[asg.Region] == nil {
				asgSizeSums[asg.Region] = make(map[int64]int)
			}
			asgSizeSums[asg.Region][asg.DesiredCapacity]++

			regionSums[asg.Region]++
			ch <- asg
		}
		for region, sum := range regionSums {
			log.Info(fmt.Sprintf("Found %d total AutoScalingGroups in %s", sum, region))
		}
		for _, e := range *events {
			for region, regionMap := range asgSizeSums {
				for asgSize, asgSizeSum := range regionMap {
					err := e.NewStatistic("reaper.asgs.asgsizes", float64(asgSizeSum), []string{fmt.Sprintf("region:%s,asgsize:%d", region, asgSize)})
					if err != nil {
						log.Error(fmt.Sprintf("%s", err.Error()))
					}
				}
			}

			for region, regionSum := range regionSums {
				err := e.NewStatistic("reaper.asgs.total", float64(regionSum), []string{fmt.Sprintf("region:%s", region)})
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
	// all resources are appended to owned or unowned
	owned := make(map[string][]reaperevents.Reapable)
	var unowned []reaperevents.Reapable

	if config.Cloudformations.Enabled {
		for c := range getCloudformationStacks() {
			// group instances by owner
			if c.Owner() != nil {
				owned[c.Owner().Address] = append(owned[c.Owner().Address], c)
			} else {
				// if unowned, append to unowned
				unowned = append(unowned, c)
			}
		}
	}

	if config.Instances.Enabled {
		// get all instances
		for i := range getInstances() {
			// group instances by owner
			if i.Owner() != nil {
				owned[i.Owner().Address] = append(owned[i.Owner().Address], i)
			} else {
				// if unowned, append to unowned
				unowned = append(unowned, i)
			}
		}
	}
	if config.AutoScalingGroups.Enabled {
		for a := range getAutoScalingGroups() {
			// group asgs by owner
			if a.Owner() != nil {
				owned[a.Owner().Address] = append(owned[a.Owner().Address], a)
			} else {
				// if unowned, append to unowned
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
		var groups map[string]filters.FilterGroup
		switch filterable.(type) {
		case *reaperaws.Instance:
			// if instances are not enabled, skip
			if !config.Instances.Enabled {
				continue
			}
			groups = config.Instances.FilterGroups
		case *reaperaws.AutoScalingGroup:
			// if ASGs are not enabled, skip
			if !config.AutoScalingGroups.Enabled {
				continue
			}
			groups = config.AutoScalingGroups.FilterGroups
		case *reaperaws.CloudformationStack:
			// if CFs are not enabled, skip
			if !config.Cloudformations.Enabled {
				continue
			}
			groups = config.Cloudformations.FilterGroups
		default:
			log.Warning("You probably screwed up and need to make sure applyFilters works!")
			return []reaperevents.Reapable{}
		}

		matched := false
		for name, group := range groups {
			didMatch := filters.ApplyFilters(filterable, group)
			if didMatch {
				matched = true
				filterable.AddFilterGroup(name, group)
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

func reapCloudformationStack(c *reaperaws.CloudformationStack) {
	// update the internal state
	if time.Now().After(c.ReaperState().Until) {
		_ = c.IncrementState()
	}
	log.Notice(fmt.Sprintf("Reapable Cloudformation discovered: %s.", c.ReapableDescription()))
	reapables.Put(c.Region, c.ID, c)
}

func reapInstance(i *reaperaws.Instance) {
	// update the internal state
	if time.Now().After(i.ReaperState().Until) {
		_ = i.IncrementState()
	}
	log.Notice(fmt.Sprintf("Reapable Instance discovered: %s.", i.ReapableDescription()))
	reapables.Put(i.Region, i.ID, i)
}

func reapAutoScalingGroup(a *reaperaws.AutoScalingGroup) {
	// update the internal state
	if time.Now().After(a.ReaperState().Until) {
		_ = a.IncrementState()
	}
	log.Notice(fmt.Sprintf("Reapable AutoScalingGroup discovered: %s.", a.ReapableDescription()))
	reapables.Put(a.Region, a.ID, a)
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
