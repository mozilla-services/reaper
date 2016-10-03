package reaper

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	reaperaws "github.com/mozilla-services/reaper/aws"
	reaperevents "github.com/mozilla-services/reaper/events"
	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/prices"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
	"github.com/robfig/cron"
)

var (
	reapables   reapable.Reapables
	savedstates map[reapable.Region]map[reapable.ID]*state.State
	config      *Config
	schedule    *cron.Cron
	pricesMap   prices.PricesMap
)

func SetConfig(c *Config) {
	config = c
}

// Ready NEEDS to be called for EventReporters and Reapables to be properly initialized
// which means events AND config need to be set BEFORE Ready
func Ready() {
	reaperevents.SetDryRun(config.DryRun)

	if r := reapable.NewReapables(config.AWS.Regions); r != nil {
		reapables = *r
	} else {
		log.Error("reapables improperly initialized")
	}
}

// Reaper finds resources and deals with them
type Reaper struct {
	*cron.Cron
}

// NewReaper is a Reaper constructor shorthand
func NewReaper() *Reaper {
	return &Reaper{
		Cron: cron.New(),
	}
}

func GetPrices() {
	// prevent shadowing
	var err error
	log.Info("Downloading prices")
	pricesMap, err = prices.DownloadPricesMap(prices.Ec2PricingUrl)
	if err != nil {
		log.Error(fmt.Sprintf("Error getting prices: %s", err.Error()))
		return
	}
	log.Info("Successfully downloaded prices")
}

// Start begins Reaper's schedule
func (r *Reaper) Start() {
	// adding as a job runs r.Run() every interval
	r.Cron.Schedule(cron.Every(config.Notifications.Interval.Duration), r)
	r.Cron.Start()

	// initial prices download, synchronous
	GetPrices()

	// initial run
	go r.Run()

	// if loading from saved state file (overriding AWS states)
	if config.LoadFromStateFile {
		r.LoadState(config.StateFile)
	}
}

// Stop stops Reaper's schedule
func (r *Reaper) Stop() {
	log.Debug("Stopping Reaper")
	reaperevents.Cleanup()
	r.Cron.Stop()
}

// Run handles all reaping logic
// conforms to the cron.Job interface
func (r *Reaper) Run() {
	schedule = cron.New()
	schedule.AddFunc("@weekly", GetPrices)
	schedule.Start()
	r.reap()

	if config.StateFile != "" {
		r.SaveState(config.StateFile)
	}

	// this is no longer true, but is roughly accurate
	log.Info("Sleeping for %s", config.Notifications.Interval.Duration.String())
}

func (r *Reaper) SaveState(stateFile string) {
	// open file RW, create it if it doesn't exist
	s, err := os.OpenFile(stateFile, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0664)
	if err != nil {
		log.Error("Unable to create StateFile ", stateFile)
		return
	}
	defer s.Close()
	// save state to state file
	for r := range reapables.Iter() {
		_, err := s.Write([]byte(fmt.Sprintf("%s,%s,%s\n",
			r.Region().String(), r.ID().String(), r.ReaperState().String())))
		if err != nil {
			log.Error("Error writing to", stateFile)
		}
	}
	log.Info("States saved to %s", stateFile)
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
			log.Error("Malformed saved state ", scanner.Text())
			continue
		}
		region := reapable.Region(line[0])
		id := reapable.ID(line[1])
		savedState := state.NewStateWithTag(line[2])

		savedstates[region][id] = savedState
	}
	if err != nil {
		log.Error("Unable to open StateFile ", stateFile)
	} else {
		log.Info("States loaded from ", stateFile)
	}
}

func (r *Reaper) reap() {
	reapables := allReapables()

	ownerMap := make(map[string][]reaperevents.Reapable)
	for _, reapable := range reapables {
		// default owner should ensure this does not happen
		if reapable.Owner() == nil {
			log.Error("Resource %s has no owner", reapable.ReapableDescriptionTiny())
			continue
		}
		// group resources by owner
		owner := reapable.Owner().Address
		ownerMap[owner] = append(ownerMap[owner], reapable)

		// TODO naively re-call matchesFilters here
		// after previously calling it for statistics
		if matchesFilters(reapable) {
			reapReapable(reapable)
		}
	}

	// trigger batch events for each filtered owned resource in a goroutine
	// for each owner in the owner map
	go func() {
		// trigger a per owner batch event
		for _, ownedReapablesArray := range ownerMap {
			// if there's only one resource for the owner, do a single event
			if len(ownedReapablesArray) == 1 {
				if err := reaperevents.NewReapableEvent(ownedReapablesArray[0], []string{config.EventTag}); err != nil {
					log.Error(err.Error())
				}
			} else {
				// batch event
				if err := reaperevents.NewBatchReapableEvent(ownedReapablesArray, []string{config.EventTag}); err != nil {
					log.Error(err.Error())
				}
			}
		}
	}()
}

func getSecurityGroups() chan *reaperaws.SecurityGroup {
	ch := make(chan *reaperaws.SecurityGroup)
	go func() {
		securityGroupCh := reaperaws.AllSecurityGroups()
		regionSums := make(map[reapable.Region]int)
		filteredCount := make(map[reapable.Region]int)
		whitelistedCount := make(map[reapable.Region]int)
		for sg := range securityGroupCh {
			// restore saved state from file
			savedstate, ok := savedstates[sg.Region()][sg.ID()]
			if ok {
				sg.SetReaperState(savedstate)
			}
			regionSums[sg.Region()]++

			if isWhitelisted(sg) {
				whitelistedCount[sg.Region()]++
			}

			if matchesFilters(sg) {
				filteredCount[sg.Region()]++
			}
			ch <- sg
		}

		for region, sum := range regionSums {
			log.Info("Found %d total SecurityGroups in %s", sum, region)
		}
		go func() {
			for region, regionSum := range regionSums {
				err := reaperevents.NewStatistic("reaper.securitygroups.total", float64(regionSum), []string{fmt.Sprintf("region:%s", region), config.EventTag})
				if err != nil {
					log.Error(err.Error())
				}
				err = reaperevents.NewStatistic("reaper.securitygroups.whitelistedCount", float64(whitelistedCount[region]), []string{fmt.Sprintf("region:%s", region), config.EventTag})
				if err != nil {
					log.Error(err.Error())
				}
				err = reaperevents.NewStatistic("reaper.securitygroups.filtered", float64(filteredCount[region]), []string{fmt.Sprintf("region:%s", region), config.EventTag})
				if err != nil {
					log.Error(err.Error())
				}
			}
		}()
		close(ch)
	}()
	return ch
}

func getVolumes() chan *reaperaws.Volume {
	ch := make(chan *reaperaws.Volume)
	go func() {
		volumeCh := reaperaws.AllVolumes()
		regionSums := make(map[reapable.Region]int)
		volumeSizeSums := make(map[reapable.Region]map[int64]int)
		filteredCount := make(map[reapable.Region]int)
		whitelistedCount := make(map[reapable.Region]int)
		for volume := range volumeCh {
			// restore saved state from file
			savedstate, ok := savedstates[volume.Region()][volume.ID()]
			if ok {
				volume.SetReaperState(savedstate)
			}

			// make the map if it is not initialized
			if volumeSizeSums[volume.Region()] == nil {
				volumeSizeSums[volume.Region()] = make(map[int64]int)
			}
			regionSums[volume.Region()]++

			if isWhitelisted(volume) {
				whitelistedCount[volume.Region()]++
			}

			volumeSizeSums[volume.Region()][*volume.Size]++

			if matchesFilters(volume) {
				filteredCount[volume.Region()]++
			}
			ch <- volume
		}

		for region, sum := range regionSums {
			log.Info("Found %d total volumes in %s", sum, region)
		}

		go func() {
			for region, regionMap := range volumeSizeSums {
				for volumeType, volumeSizeSum := range regionMap {
					err := reaperevents.NewStatistic("reaper.volumes.total", float64(volumeSizeSum), []string{fmt.Sprintf("region:%s,volumesize:%d", region, volumeType)})
					if err != nil {
						log.Error(err.Error())
					}
					err = reaperevents.NewStatistic("reaper.volumes.filtered", float64(filteredCount[region]), []string{fmt.Sprintf("region:%s,volumesize:%d", region, volumeType)})
					if err != nil {
						log.Error(err.Error())
					}
					err = reaperevents.NewStatistic("reaper.volumes.whitelistedCount", float64(whitelistedCount[region]), []string{fmt.Sprintf("region:%s,volumesize:%d", region, volumeType)})
					if err != nil {
						log.Error(err.Error())
					}
				}
			}
		}()
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
		filteredCount := make(map[reapable.Region]int)
		whitelistedCount := make(map[reapable.Region]int)
		for instance := range instanceCh {
			// restore saved state from file
			savedstate, ok := savedstates[instance.Region()][instance.ID()]
			if ok {
				instance.SetReaperState(savedstate)
			}

			// make the map if it is not initialized
			if instanceTypeSums[instance.Region()] == nil {
				instanceTypeSums[instance.Region()] = make(map[string]int)
			}

			// don't count terminated or stopped instances
			if !instance.Terminated() && !instance.Stopped() {
				// increment InstanceType counter
				instanceTypeSums[instance.Region()][*instance.InstanceType]++

				if isWhitelisted(instance) {
					whitelistedCount[instance.Region()]++
				}
			}

			regionSums[instance.Region()]++

			if matchesFilters(instance) {
				filteredCount[instance.Region()]++
			}
			ch <- instance
		}

		for region, sum := range regionSums {
			log.Info("Found %d total Instances in %s", sum, region)
		}

		go func() {
			for region, regionMap := range instanceTypeSums {
				for instanceType, instanceTypeSum := range regionMap {
					if pricesMap != nil {
						price, ok := pricesMap[string(region)][instanceType]
						if ok {
							priceFloat, err := strconv.ParseFloat(price, 64)
							if err != nil {
								log.Error(err.Error())
							}
							err = reaperevents.NewStatistic("reaper.instances.totalcost", float64(instanceTypeSum)*priceFloat, []string{fmt.Sprintf("region:%s,instancetype:%s", region, instanceType), config.EventTag})
							if err != nil {
								log.Error(err.Error())
							}
						} else {
							// some instance types are priceless
							log.Error(fmt.Sprintf("No price for %s", instanceType))
						}
					}
					err := reaperevents.NewStatistic("reaper.instances.total", float64(instanceTypeSum), []string{fmt.Sprintf("region:%s,instancetype:%s", region, instanceType), config.EventTag})
					if err != nil {
						log.Error(err.Error())
					}
					err = reaperevents.NewStatistic("reaper.instances.filtered", float64(filteredCount[region]), []string{fmt.Sprintf("region:%s,instancetype:%s", region, instanceType), config.EventTag})
					if err != nil {
						log.Error(err.Error())
					}
					err = reaperevents.NewStatistic("reaper.instances.whitelistedCount", float64(whitelistedCount[region]), []string{fmt.Sprintf("region:%s,instancetype:%s", region, instanceType), config.EventTag})
					if err != nil {
						log.Error(err.Error())
					}
				}
			}
		}()
		close(ch)
	}()
	return ch
}

func getCloudformations() chan *reaperaws.Cloudformation {
	ch := make(chan *reaperaws.Cloudformation)
	go func() {
		cfs := reaperaws.AllCloudformations()
		regionSums := make(map[reapable.Region]int)
		filteredCount := make(map[reapable.Region]int)
		whitelistedCount := make(map[reapable.Region]int)
		for cf := range cfs {
			// restore saved state from file
			savedstate, ok := savedstates[cf.Region()][cf.ID()]
			if ok {
				cf.SetReaperState(savedstate)
			}

			if isWhitelisted(cf) {
				whitelistedCount[cf.Region()]++
			}
			regionSums[cf.Region()]++

			if matchesFilters(cf) {
				filteredCount[cf.Region()]++
			}
			ch <- cf
		}
		for region, sum := range regionSums {
			log.Info("Found %d total Cloudformation Stacks in %s", sum, region)
		}
		go func() {
			for region, regionSum := range regionSums {
				err := reaperevents.NewStatistic("reaper.cloudformations.total", float64(regionSum), []string{fmt.Sprintf("region:%s", region), config.EventTag})
				if err != nil {
					log.Error(err.Error())
				}
				err = reaperevents.NewStatistic("reaper.cloudformations.filtered", float64(filteredCount[region]), []string{fmt.Sprintf("region:%s", region), config.EventTag})
				if err != nil {
					log.Error(err.Error())
				}
				err = reaperevents.NewStatistic("reaper.cloudformations.whitelistedCount", float64(whitelistedCount[region]), []string{fmt.Sprintf("region:%s", region), config.EventTag})
				if err != nil {
					log.Error(err.Error())
				}
			}
		}()
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
		filteredCount := make(map[reapable.Region]int)
		whitelistedCount := make(map[reapable.Region]int)
		for asg := range asgCh {
			// restore saved state from file
			savedstate, ok := savedstates[asg.Region()][asg.ID()]
			if ok {
				asg.SetReaperState(savedstate)
			}

			// make the map if it is not initialized
			if asgSizeSums[asg.Region()] == nil {
				asgSizeSums[asg.Region()] = make(map[int64]int)
			}
			if asg.DesiredCapacity != nil {
				asgSizeSums[asg.Region()][*asg.DesiredCapacity]++
			}

			if isWhitelisted(asg) {
				whitelistedCount[asg.Region()]++
			}

			regionSums[asg.Region()]++

			if matchesFilters(asg) {
				filteredCount[asg.Region()]++
			}
			ch <- asg
		}
		for region, sum := range regionSums {
			log.Info("Found %d total AutoScalingGroups in %s", sum, region)
		}
		go func() {
			for region, regionMap := range asgSizeSums {
				for asgSize, asgSizeSum := range regionMap {
					err := reaperevents.NewStatistic("reaper.asgs.asgsizes", float64(asgSizeSum), []string{fmt.Sprintf("region:%s,asgsize:%d", region, asgSize), config.EventTag})
					if err != nil {
						log.Error(err.Error())
					}
				}
			}
			for region, regionSum := range regionSums {
				err := reaperevents.NewStatistic("reaper.asgs.total", float64(regionSum), []string{fmt.Sprintf("region:%s", region), config.EventTag})
				if err != nil {
					log.Error(err.Error())
				}
				err = reaperevents.NewStatistic("reaper.asgs.filtered", float64(filteredCount[region]), []string{fmt.Sprintf("region:%s", region), config.EventTag})
				if err != nil {
					log.Error(err.Error())
				}
				err = reaperevents.NewStatistic("reaper.asgs.whitelistedCount", float64(whitelistedCount[region]), []string{fmt.Sprintf("region:%s", region), config.EventTag})
				if err != nil {
					log.Error(err.Error())
				}
			}

		}()
		close(ch)
	}()
	return ch
}

// makes a slice of all filterables by appending
// output of each filterable types aggregator function
func allReapables() []reaperevents.Reapable {
	var resources []reaperevents.Reapable

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

	// without getCloudformations cannot populate basic dependency logic
	for c := range getCloudformations() {
		// because getting resources is rate limited...
		c.RLock()
		defer c.RUnlock()
		for _, resource := range c.Resources {
			if resource.PhysicalResourceId != nil {
				dependency[c.Region()][reapable.ID(*resource.PhysicalResourceId)] = true
				isInCloudformation[c.Region()][reapable.ID(*resource.PhysicalResourceId)] = true
			}
		}

		if config.Cloudformations.Enabled {
			resources = append(resources, c)
		}
	}

	for a := range getAutoScalingGroups() {
		// ASGs can be identified by name...
		if isInCloudformation[a.Region()][a.ID()] ||
			isInCloudformation[a.Region()][reapable.ID(a.Name)] {
			a.IsInCloudformation = true
		}

		if dependency[a.Region()][a.ID()] ||
			dependency[a.Region()][reapable.ID(a.Name)] {
			a.Dependency = true
		}

		if a.Scheduling.Enabled {
			if log.Extras() {
				log.Info("AutoScalingGroup %s is going to be scaled down: %s and scaled up: %s.", a.ID().String(), a.Scheduling.ScaleDownString, a.Scheduling.ScaleUpString)
			}
			schedule.AddFunc(a.Scheduling.ScaleDownString, a.ScaleDown)
			schedule.AddFunc(a.Scheduling.ScaleUpString, a.ScaleUp)
		}

		// identify instances in an ASG
		instanceIDsInASGs := reaperaws.AutoScalingGroupInstanceIDs(a)
		for region := range instanceIDsInASGs {
			for instanceID := range instanceIDsInASGs[region] {
				instancesInASGs[region][instanceID] = true
				dependency[region][instanceID] = true
			}
		}

		if config.AutoScalingGroups.Enabled {
			resources = append(resources, a)
		}
	}

	// get all instances
	for i := range getInstances() {
		// add security groups to map of in use
		for id, name := range i.SecurityGroups {
			dependency[i.Region()][reapable.ID(name)] = true
			dependency[i.Region()][id] = true
		}

		if dependency[i.Region()][i.ID()] {
			i.Dependency = true
		}
		if isInCloudformation[i.Region()][i.ID()] {
			i.IsInCloudformation = true
		}
		if instancesInASGs[i.Region()][i.ID()] {
			i.AutoScaled = true
		}

		if i.Scheduling.Enabled {
			if log.Extras() {
				log.Info("Instance %s is going to be scaled down: %s and scaled up: %s.", i.ID().String(), i.Scheduling.ScaleDownString, i.Scheduling.ScaleUpString)
			}
			schedule.AddFunc(i.Scheduling.ScaleDownString, i.ScaleDown)
			schedule.AddFunc(i.Scheduling.ScaleUpString, i.ScaleUp)
		}

		if config.Instances.Enabled {
			resources = append(resources, i)
		}
	}

	// get all security groups
	for s := range getSecurityGroups() {
		// if the security group is in use, it isn't reapable
		// names and IDs are used interchangeably by different parts of the API
		if isInCloudformation[s.Region()][s.ID()] {
			s.IsInCloudformation = true
		}
		if dependency[s.Region()][s.ID()] ||
			dependency[s.Region()][reapable.ID(*s.GroupName)] {
			s.Dependency = true
		}
		if config.SecurityGroups.Enabled {
			resources = append(resources, s)
		}
	}

	// get all the volumes
	for v := range getVolumes() {
		// if the volume is in use, it isn't reapable
		// names and IDs are used interchangeably by different parts of the API

		// sort of doesn't make sense for volume
		if isInCloudformation[v.Region()][v.ID()] {
			v.IsInCloudformation = true
		}

		// if it is a dependency or is attached to an instance
		if dependency[v.Region()][v.ID()] || len(v.AttachedInstanceIDs) > 0 {
			v.Dependency = true
		}
		if config.Volumes.Enabled {
			resources = append(resources, v)
		}
	}
	return resources
}

// isWhitelisted returns whether the filterable is tagged
// with the whitelist tag
func isWhitelisted(filterable filters.Filterable) bool {
	return filterable.Filter(*filters.NewFilter("Tagged", []string{config.WhitelistTag}))
}

// matchesFilters applies the relevant filter groups to a filterable
func matchesFilters(filterable filters.Filterable) bool {
	// recover from potential panics caused by malformed filters
	defer func() {
		if r := recover(); r != nil {
			log.Error("Recovered in matchesFilters with panic: ", r)
		}
	}()

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
	case *reaperaws.Volume:
		groups = config.Volumes.FilterGroups
	default:
		log.Warning("You probably screwed up and need to make sure matchesFilters works!")
		return false
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

	// convenient
	if isWhitelisted(filterable) {
		matched = false
	}

	return matched
}

func reapReapable(a reaperevents.Reapable) {
	// update the internal state
	if time.Now().After(a.ReaperState().Until) {
		// if we updated the state, mark it as having been updated
		a.SetUpdated(a.IncrementState())
	}
	log.Info("Reapable resource discovered: %s.", a.ReapableDescription())
	reapables.Put(a.Region(), a.ID(), a)
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
	log.Debug("Stop ", reapable.ReapableDescriptionShort())

	return nil
}
