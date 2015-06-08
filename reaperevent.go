package main

import (
	"time"
)

type ReaperEventConfig struct {
	Enabled bool
}

type ReaperEvent struct {
	Config *ReaperEventConfig
}

func (r *ReaperEvent) NewEvent(title string, text string, fields map[string]string, tags []string) {}
func (r *ReaperEvent) NewStatistic(name string, value float64, tags []string)                      {}
func (r *ReaperEvent) NewCountStatistic(name string, tags []string)                                {}
func (r *ReaperEvent) NewReapableInstanceEvent(i *Instance) {
	// this only gets called if ReaperEvent is added, so we check
	// for dryrun, that we have passed NOTIFY2, and that current time is
	// later than the Until time
	if !Conf.DryRun && r.Config.Enabled &&
		time.Now().After(i.ReaperState.Until) &&
		i.ReaperState.State == STATE_REAPABLE {
		Log.Notice("ReaperEvent is stopping %s in region %s.", i.ID, i.Region)
		_, err := i.Stop()
		if err != nil {
			Log.Error("%s", err.Error())
		}
	}
}

func (r *ReaperEvent) NewReapableASGEvent(a *AutoScalingGroup) {
	// this only gets called if ReaperEvent is added, so we check
	// for dryrun, that we have passed NOTIFY2, and that current time is
	// later than the Until time
	if !Conf.DryRun && r.Config.Enabled &&
		time.Now().After(a.ReaperState.Until) &&
		a.ReaperState.State == STATE_REAPABLE {
		_, err := a.Stop()
		if err != nil {
			Log.Error("%s", err.Error())
		}
	}
}
