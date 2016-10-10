package reaper

import (
	"fmt"
	"strconv"

	reaperaws "github.com/mozilla-services/reaper/aws"
	reaperevents "github.com/mozilla-services/reaper/events"
	log "github.com/mozilla-services/reaper/reaperlog"

	"github.com/mozilla-services/reaper/reapable"
)

func getSecurityGroups() chan *reaperaws.SecurityGroup {
	ch := make(chan *reaperaws.SecurityGroup)
	go func() {
		securityGroupCh := reaperaws.AllSecurityGroups()
		regionSums := make(map[reapable.Region]int)
		filteredCount := make(map[reapable.Region]int)
		whitelistedCount := make(map[reapable.Region]int)
		for sg := range securityGroupCh {
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
				err := reaperevents.NewStatistic("reaper.securitygroups.total",
					float64(regionSum),
					[]string{fmt.Sprintf("region:%s", region), config.EventTag})
				if err != nil {
					log.Error(err.Error())
				}
				err = reaperevents.NewStatistic("reaper.securitygroups.whitelistedCount",
					float64(whitelistedCount[region]),
					[]string{fmt.Sprintf("region:%s", region), config.EventTag})
				if err != nil {
					log.Error(err.Error())
				}
				err = reaperevents.NewStatistic("reaper.securitygroups.filtered",
					float64(filteredCount[region]),
					[]string{fmt.Sprintf("region:%s", region), config.EventTag})
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
					err := reaperevents.NewStatistic("reaper.volumes.total",
						float64(volumeSizeSum),
						[]string{fmt.Sprintf("region:%s,volumesize:%d", region, volumeType)})
					if err != nil {
						log.Error(err.Error())
					}
					err = reaperevents.NewStatistic("reaper.volumes.filtered",
						float64(filteredCount[region]),
						[]string{fmt.Sprintf("region:%s,volumesize:%d", region, volumeType)})
					if err != nil {
						log.Error(err.Error())
					}
					err = reaperevents.NewStatistic("reaper.volumes.whitelistedCount",
						float64(whitelistedCount[region]),
						[]string{fmt.Sprintf("region:%s,volumesize:%d", region, volumeType)})
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
							err = reaperevents.NewStatistic("reaper.instances.totalcost",
								float64(instanceTypeSum)*priceFloat,
								[]string{fmt.Sprintf("region:%s,instancetype:%s", region, instanceType), config.EventTag})
							if err != nil {
								log.Error(err.Error())
							}
						} else {
							// some instance types are priceless
							log.Error(fmt.Sprintf("No price for %s", instanceType))
						}
					}
					err := reaperevents.NewStatistic("reaper.instances.total",
						float64(instanceTypeSum),
						[]string{fmt.Sprintf("region:%s,instancetype:%s", region, instanceType), config.EventTag})
					if err != nil {
						log.Error(err.Error())
					}
					err = reaperevents.NewStatistic("reaper.instances.filtered",
						float64(filteredCount[region]),
						[]string{fmt.Sprintf("region:%s,instancetype:%s", region, instanceType), config.EventTag})
					if err != nil {
						log.Error(err.Error())
					}
					err = reaperevents.NewStatistic("reaper.instances.whitelistedCount",
						float64(whitelistedCount[region]),
						[]string{fmt.Sprintf("region:%s,instancetype:%s", region, instanceType), config.EventTag})
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
				err := reaperevents.NewStatistic("reaper.cloudformations.total",
					float64(regionSum),
					[]string{fmt.Sprintf("region:%s", region), config.EventTag})
				if err != nil {
					log.Error(err.Error())
				}
				err = reaperevents.NewStatistic("reaper.cloudformations.filtered",
					float64(filteredCount[region]),
					[]string{fmt.Sprintf("region:%s", region), config.EventTag})
				if err != nil {
					log.Error(err.Error())
				}
				err = reaperevents.NewStatistic("reaper.cloudformations.whitelistedCount",
					float64(whitelistedCount[region]),
					[]string{fmt.Sprintf("region:%s", region), config.EventTag})
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
					err := reaperevents.NewStatistic("reaper.asgs.asgsizes",
						float64(asgSizeSum),
						[]string{fmt.Sprintf("region:%s,asgsize:%d", region, asgSize), config.EventTag})
					if err != nil {
						log.Error(err.Error())
					}
				}
			}
			for region, regionSum := range regionSums {
				err := reaperevents.NewStatistic("reaper.asgs.total",
					float64(regionSum),
					[]string{fmt.Sprintf("region:%s", region), config.EventTag})
				if err != nil {
					log.Error(err.Error())
				}
				err = reaperevents.NewStatistic("reaper.asgs.filtered",
					float64(filteredCount[region]),
					[]string{fmt.Sprintf("region:%s", region), config.EventTag})
				if err != nil {
					log.Error(err.Error())
				}
				err = reaperevents.NewStatistic("reaper.asgs.whitelistedCount",
					float64(whitelistedCount[region]),
					[]string{fmt.Sprintf("region:%s", region), config.EventTag})
				if err != nil {
					log.Error(err.Error())
				}
			}

		}()
		close(ch)
	}()
	return ch
}
