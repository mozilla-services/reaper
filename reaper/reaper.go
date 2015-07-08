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
	filteredSecurityGroupSums := make(map[reapable.Region]int)
	filteredCloudformationSums := make(map[reapable.Region]int)

	// filtered has _all_ resources post filtering
	for _, f := range filtered {
		switch t := f.(type) {
		case *reaperaws.Instance:
			filteredInstanceSums[t.Region]++
			reapInstance(t)
		case *reaperaws.AutoScalingGroup:
			filteredASGSums[t.Region]++
			reapAutoScalingGroup(t)
		case *reaperaws.SecurityGroup:
			filteredSecurityGroupSums[t.Region]++
			reapSecurityGroup(t)
		case *reaperaws.Cloudformation:
			filteredCloudformationSums[t.Region]++
			reapCloudformation(t)
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
		for region, sum := range filteredCloudformationSums {
			err := e.NewStatistic("reaper.cloudformations.filtered", float64(sum), []string{fmt.Sprintf("region:%s", region)})
			if err != nil {
				log.Error(fmt.Sprintf("%s", err.Error()))
			}
		}
		for region, sum := range filteredSecurityGroupSums {
			err := e.NewStatistic("reaper.securitygroups.filtered", float64(sum), []string{fmt.Sprintf("region:%s", region)})
			if err != nil {
				log.Error(fmt.Sprintf("%s", err.Error()))
			}
		}
	}
}

func getSecurityGroups() chan *reaperaws.SecurityGroup {
	ch := make(chan *reaperaws.SecurityGroup)
	go func() {
		securityGroupCh := reaperaws.AllSecurityGroups()
		regionSums := make(map[reapable.Region]int)
		for securityGroup := range securityGroupCh {
			// restore saved state from file
			savedstate, ok := savedstates[securityGroup.Region][securityGroup.ID]
			if ok {
				securityGroup.SetReaperState(savedstate)
			}
			regionSums[securityGroup.Region]++
			ch <- securityGroup
		}

		for region, sum := range regionSums {
			log.Info(fmt.Sprintf("Found %d total SecurityGroups in %s", sum, region))
		}
		for _, e := range *events {
			for region, regionSum := range regionSums {
				err := e.NewStatistic("reaper.securitygroups.total", float64(regionSum), []string{fmt.Sprintf("region:%s", region)})
				if err != nil {
					log.Error(fmt.Sprintf("%s", err.Error()))
				}
			}
		}
		close(ch)
	}()
	return ch
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
				if instance.InstanceType != nil {
					// increment InstanceType counter
					instanceTypeSums[instance.Region][*instance.InstanceType]++
				}
			}

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

func getCloudformations() chan *reaperaws.Cloudformation {
	ch := make(chan *reaperaws.Cloudformation)
	go func() {
		cfs := reaperaws.AllCloudformations()
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
			if asg.DesiredCapacity != nil {
				asgSizeSums[asg.Region][*asg.DesiredCapacity]++
			}

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

	// initialize dependency and isInCloudformation
	dependency := make(map[reapable.Region]map[reapable.ID]bool)
	for _, region := range config.AWS.Regions {
		dependency[reapable.Region(region)] = make(map[reapable.ID]bool)
	}

	isInCloudformation := make(map[reapable.Region]map[reapable.ID]bool)
	for _, region := range config.AWS.Regions {
		isInCloudformation[reapable.Region(region)] = make(map[reapable.ID]bool)
	}

	// initialize the map of instances in ASGs
	instancesInASGs := make(map[reapable.Region]map[reapable.ID]bool)
	for _, region := range config.AWS.Regions {
		instancesInASGs[reapable.Region(region)] = make(map[reapable.ID]bool)
	}

	for c := range getCloudformations() {
		// because getting resources is rate limited...
		c.RLock()
		defer c.RUnlock()
		for _, resource := range c.Resources {
			if resource.PhysicalResourceID != nil {
				dependency[c.Region][reapable.ID(*resource.PhysicalResourceID)] = true
				isInCloudformation[c.Region][reapable.ID(*resource.PhysicalResourceID)] = true
			}
		}
		if config.Cloudformations.Enabled {
			// group CFs by owner
			if c.Owner() != nil {
				owned[c.Owner().Address] = append(owned[c.Owner().Address], c)
			} else {
				// if unowned, append to unowned
				unowned = append(unowned, c)
			}
		}
	}

	for a := range getAutoScalingGroups() {
		// ASGs can be identified by name...
		if isInCloudformation[a.Region][a.ID] || isInCloudformation[a.Region][reapable.ID(a.Name)] {
			a.IsInCloudformation = true
		}

		// identify instances in an ASG
		instanceIDsInASGs := reaperaws.ASGInstanceIDs(a)
		for region := range instanceIDsInASGs {
			for instanceID := range instanceIDsInASGs[region] {
				instancesInASGs[region][instanceID] = true
			}
		}

		// group asgs by owner
		if config.AutoScalingGroups.Enabled {
			if a.Owner() != nil {
				owned[a.Owner().Address] = append(owned[a.Owner().Address], a)
			} else {
				// if unowned, append to unowned
				unowned = append(unowned, a)
			}
		}
	}

	// get all instances
	for i := range getInstances() {
		// add security groups to map of in use
		for id, name := range i.SecurityGroups {
			dependency[i.Region][reapable.ID(name)] = true
			dependency[i.Region][id] = true
		}

		if isInCloudformation[i.Region][i.ID] {
			i.IsInCloudformation = true
		}
		if instancesInASGs[i.Region][i.ID] {
			i.AutoScaled = true
		}
		// group instances by owner
		if config.Instances.Enabled {
			if i.Owner() != nil {
				owned[i.Owner().Address] = append(owned[i.Owner().Address], i)
			} else {
				// if unowned, append to unowned
				unowned = append(unowned, i)
			}
		}
	}

	// get all security groups
	for s := range getSecurityGroups() {
		// if the security group is in use, it isn't reapable
		// names and IDs are used interchangeably by different parts of the API
		if isInCloudformation[s.Region][s.ID] {
			s.IsInCloudformation = true
		}
		if dependency[s.Region][s.ID] || dependency[s.Region][reapable.ID(*s.GroupName)] {
			continue
		}
		if config.SecurityGroups.Enabled {
			// group instances by owner
			if s.Owner() != nil {
				owned[s.Owner().Address] = append(owned[s.Owner().Address], s)
			} else {
				// if unowned, append to unowned
				unowned = append(unowned, s)
			}
		}
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
			groups = config.Instances.FilterGroups
		case *reaperaws.AutoScalingGroup:
			groups = config.AutoScalingGroups.FilterGroups
		case *reaperaws.Cloudformation:
			groups = config.Cloudformations.FilterGroups
		case *reaperaws.SecurityGroup:
			groups = config.SecurityGroups.FilterGroups
		default:
			log.Warning("You probably screwed up and need to make sure applyFilters works!")
			return []reaperevents.Reapable{}
		}

		matched := false

		// if there are no filters groups defined, default to a match
		if len(groups) == 0 {
			matched = true
		}

		// if there are no filters, default to a match
		noFilters := true
		for _, group := range groups {
			if len(group) != 0 {
				// set to false if any are non-zero length
				noFilters = false
			}
		}
		if noFilters {
			matched = true
		}

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

func reapSecurityGroup(s *reaperaws.SecurityGroup) {
	// update the internal state
	if time.Now().After(s.ReaperState().Until) {
		_ = s.IncrementState()
	}
	log.Notice(fmt.Sprintf("Reapable SecurityGroup discovered: %s.", s.ReapableDescription()))
	reapables.Put(s.Region, s.ID, s)
}

func reapCloudformation(c *reaperaws.Cloudformation) {
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
		log.Error(fmt.Sprintf("Could not terminate resource with region: %s and id: %s. Error: %s",
			region, id, err.Error()))
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
		log.Error(fmt.Sprintf("Could not stop resource with region: %s and id: %s. Error: %s",
			region, id, err.Error()))
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
		log.Error(fmt.Sprintf("Could not stop resource with region: %s and id: %s. Error: %s",
			region, id, err.Error()))
		return err
	}
	log.Debug("Stop %s", reapable.ReapableDescriptionShort())

	return nil
}
