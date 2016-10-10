package reaper

import (
	"fmt"
	"time"

	reaperaws "github.com/mozilla-services/reaper/aws"
	reaperevents "github.com/mozilla-services/reaper/events"
	"github.com/mozilla-services/reaper/filters"
	"github.com/mozilla-services/reaper/prices"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/robfig/cron"
)

var (
	reapables reapable.Reapables
	config    *Config
	schedule  *cron.Cron
	pricesMap prices.PricesMap
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
	r.Cron.AddFunc("@weekly", GetPrices)
	r.Cron.Start()

	// initial prices download, synchronous
	GetPrices()

	// initial run
	go r.Run()
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
	r.reap()

	// this is no longer true, but is roughly accurate
	log.Info("Sleeping for %s", config.Notifications.Interval.Duration.String())
}

func (r *Reaper) reap() {
	reapables := allReapables()

	filteredOwnerMap := make(map[string][]reaperevents.Reapable)
	for _, reapable := range reapables {
		// default owner should ensure this does not happen
		if reapable.Owner() == nil {
			log.Error("Resource %s has no owner", reapable.ReapableDescriptionTiny())
			continue
		}

		// TODO naively re-call matchesFilters here
		// after previously calling it for statistics
		if matchesFilters(reapable) {
			// group resources by owner
			owner := reapable.Owner().Address
			filteredOwnerMap[owner] = append(filteredOwnerMap[owner], reapable)
			registerReapable(reapable)
		}
	}

	// trigger batch events for each filtered owned resource in a goroutine
	// for each owner in the owner map
	go func() {
		// trigger a per owner batch event
		for _, filteredOwnedReapables := range filteredOwnerMap {
			// if there's only one resource for the owner, do a single event
			if len(filteredOwnedReapables) == 1 {
				if err := reaperevents.NewReapableEvent(filteredOwnedReapables[0], []string{config.EventTag}); err != nil {
					log.Error(err.Error())
				}
			} else {
				// batch event
				if err := reaperevents.NewBatchReapableEvent(filteredOwnedReapables, []string{config.EventTag}); err != nil {
					log.Error(err.Error())
				}
			}
		}
	}()
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
		for _, resource := range c.Resources {
			if resource.PhysicalResourceId != nil {
				dependency[c.Region()][reapable.ID(*resource.PhysicalResourceId)] = true
				isInCloudformation[c.Region()][reapable.ID(*resource.PhysicalResourceId)] = true
			}
		}
		c.RUnlock()
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

	// if there are no filters groups defined, match
	if len(groups) == 0 {
		return true
	}

	shouldFilter := false
	for _, group := range groups {
		if len(group) > 0 {
			// there is a filter
			shouldFilter = true
		}
	}
	// no filters, default to a match
	if !shouldFilter {
		return true
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

func registerReapable(a reaperevents.Reapable) {
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
