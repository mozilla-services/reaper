package main

import "time"

type TaggerConfig struct {
	Enabled bool
}

type Tagger struct {
	Config *TaggerConfig
}

func (t *Tagger) NewEvent(title string, text string, fields map[string]string, tags []string) {}
func (t *Tagger) NewStatistic(name string, value float64, tags []string)                      {}
func (t *Tagger) NewCountStatistic(name string, tags []string)                                {}
func (t *Tagger) NewReapableInstanceEvent(i *Instance) {
	// TODO: decide whether or not we update tags on a dry run
	// if !Conf.DryRun {
	if time.Now().Before(i.ReaperState.Until) {
		// if it is before the time we increment state at
		if !i.Tagged(reaper_tag) {
			i.TagReaperState(i.ReaperState)
		}
		Log.Info("Set Reaper start state on %s in region %s. New tag: %s.", i.ID, i.Region, i.ReaperState.String())
		return
	}
	updated := i.incrementState()
	if updated {
		Log.Info("Updating tag on %s in region %s. New tag: %s.", i.ID, i.Region, i.ReaperState.String())
	}
	_, err := i.TagReaperState(i.ReaperState)
	if err != nil {
		Log.Error("%s", err.Error())
	}
	// }
}
func (t *Tagger) NewReapableASGEvent(a *AutoScalingGroup) {
	// TODO: decide whether or not we update tags on a dry run
	// if !Conf.DryRun {
	if time.Now().Before(a.ReaperState.Until) {
		// if it is before the time we increment state at
		if !a.Tagged(reaper_tag) {
			a.TagReaperState(a.ReaperState)
		}
		Log.Info("Set Reaper start state on %s in region %s. New tag: %s.", a.ID, a.Region, a.ReaperState.String())
		return
	}
	updated := a.incrementState()
	if updated {
		Log.Info("Updating tag on %s in region %s. New tag: %s.", a.ID, a.Region, a.ReaperState.String())
	}
	a.TagReaperState(a.ReaperState)
	// }
}
