package reaper

import (
	"fmt"
	"time"

	"github.com/awslabs/aws-sdk-go/aws"
	raws "github.com/mostlygeek/reaper/aws"
	"github.com/mostlygeek/reaper/filter"
	"github.com/tj/go-debug"
)

var (
	debugReaper = debug.Debug("reaper:reaper")
)

type Reaper struct {
	conf   Config
	mailer *Mailer
	log    *Logger
	dryrun bool

	stopCh chan struct{}
}

func NewReaper(c Config, m *Mailer, l *Logger) *Reaper {
	return &Reaper{
		conf:   c,
		mailer: m,
		log:    l,
		stopCh: make(chan struct{}),
	}
}

func (r *Reaper) DryRunOn()  { r.dryrun = true }
func (r *Reaper) DryRunOff() { r.dryrun = false }

func (r *Reaper) Start() {
	go r.start()
}

func (r *Reaper) Stop() {
	defer close(r.stopCh)
}

func (r *Reaper) start() {
	// make a list of all eligible instances
	for {
		r.Once()
		debugReaper("Sleeping for %s", r.conf.Reaper.Interval.Duration.String())
		select {
		case <-time.After(r.conf.Reaper.Interval.Duration):
		case <-r.stopCh: // time to exit!
			debugReaper("Stopping reaper")
			return
		}
	}
}

func (r *Reaper) awsCreds() aws.CredentialsProvider {
	return aws.DetectCreds(
		r.conf.AWS.AccessID,
		r.conf.AWS.AccessSecret,
		r.conf.AWS.Token,
	)
}

func (r *Reaper) Once() {
	// make a list of all eligible instances
	creds := r.awsCreds()

	endpoints := raws.NewEndpoints(creds, r.conf.AWS.Regions, nil)
	instances := raws.AllInstances(endpoints)

	filtered := instances.
		Filter(filter.Running).
		Filter(filter.Not(filter.Tagged("REAPER_SPARE_ME"))).
		Filter(filter.ReaperReady(r.conf.Reaper.FirstNotification.Duration)).
		Filter(filter.Tagged("REAP_ME"))

	r.log.Info(fmt.Sprintf("Found %d instances", len(filtered)))
	for _, i := range filtered {
		// determine what to do next based on the last state

		// terminate the instance if we can't determine the owner
		if i.Owner() == nil {
			r.terminateUnowned(i)
			continue
		}

		switch i.Reaper().State {
		case raws.STATE_START, raws.STATE_IGNORE:
			r.sendNotification(i, 1)
		case raws.STATE_NOTIFY1:
			r.sendNotification(i, 2)
		case raws.STATE_NOTIFY2:
			r.terminate(i)
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

func (r *Reaper) terminateUnowned(i *raws.Instance) error {
	r.info("Terminate UNOWNED instance (%s) %s, owner tag: %s",
		i.Id(), i.Name(), i.Tag("Owner"))

	if r.dryrun {
		return nil
	}

	if err := raws.Terminate(r.awsCreds(), i.Region(), i.Id()); err != nil {
		r.log.Error(fmt.Sprintf("Terminate %s error: %s", i.Id(), err.Error()))
		return err
	}

	return nil

}

func (r *Reaper) terminate(i *raws.Instance) error {
	r.info("TERMINATE %s notify2 => terminate", i.Id())
	if err := raws.Terminate(r.awsCreds(), i.Region(), i.Id()); err != nil {
		r.log.Error(fmt.Sprintf("%s failed to terminate error: %s",
			i.Id()), err.Error())
		return err
	}
	return nil
}

func (r *Reaper) sendNotification(i *raws.Instance, notifyNum int) error {
	r.info("Send Notification #%d %s", notifyNum, i.Id())
	if r.dryrun {
		return nil
	}

	var newState raws.StateEnum
	var until time.Time
	switch notifyNum {
	case 2:
		newState = raws.STATE_NOTIFY2
		until = time.Now().Add(r.conf.Reaper.Terminate.Duration)
	default:
		newState = raws.STATE_NOTIFY1
		until = time.Now().Add(r.conf.Reaper.SecondNotification.Duration)
	}

	// TODO: Send the notification here..
	if err := r.mailer.Notify(notifyNum, i); err != nil {
		debugReaper("Notify %d for %s error %s", notifyNum, i.Id(), err.Error())
		return err
	}

	err := raws.UpdateReaperState(r.awsCreds(), i.Region(), i.Id(), &raws.State{
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
