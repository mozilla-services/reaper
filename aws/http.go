package aws

import (
	"fmt"
	"net/url"
	"time"

	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/token"
)

// MakeScheduleLink creates a tokenized link scheduling a resource
func MakeScheduleLink(region reapable.Region, id reapable.ID, tokenSecret, apiURL, scaleDownSchedule, scaleUpSchedule string) (string, error) {
	sched, err := token.Tokenize(tokenSecret,
		token.NewScheduleJob(region.String(), id.String(), scaleDownSchedule, scaleUpSchedule))

	if err != nil {
		return "", err
	}

	return makeURL(apiURL, "schedule", sched), nil
}

// MakeTerminateLink creates a tokenized link scheduling a resource
func MakeTerminateLink(region reapable.Region, id reapable.ID, tokenSecret, apiURL string) (string, error) {
	term, err := token.Tokenize(tokenSecret,
		token.NewTerminateJob(id.String(), id.String()))

	if err != nil {
		return "", err
	}

	return makeURL(apiURL, "terminate", term), nil
}

// MakeIgnoreLink creates a tokenized link scheduling a resource
func MakeIgnoreLink(region reapable.Region, id reapable.ID, tokenSecret, apiURL string,
	duration time.Duration) (string, error) {
	delay, err := token.Tokenize(tokenSecret,
		token.NewDelayJob(id.String(), id.String(),
			duration))

	if err != nil {
		return "", err
	}

	action := "delay_" + duration.String()
	return makeURL(apiURL, action, delay), nil

}

// MakeWhitelistLink creates a tokenized link scheduling a resource
func MakeWhitelistLink(region reapable.Region, id reapable.ID, tokenSecret, apiURL string) (string, error) {
	whitelist, err := token.Tokenize(tokenSecret,
		token.NewWhitelistJob(region.String(), id.String()))
	if err != nil {
		log.Error(fmt.Sprintf("Error creating whitelist link: %s", err))
		return "", err
	}

	return makeURL(apiURL, "whitelist", whitelist), nil
}

// MakeStopLink creates a tokenized link scheduling a resource
func MakeStopLink(region reapable.Region, id reapable.ID, tokenSecret, apiURL string) (string, error) {
	stop, err := token.Tokenize(tokenSecret,
		token.NewStopJob(region.String(), id.String()))
	if err != nil {
		log.Error(fmt.Sprintf("Error creating ScaleToZero link: %s", err))
		return "", err
	}

	return makeURL(apiURL, "stop", stop), nil
}

// MakeForceStopLink creates a tokenized link scheduling a resource
func MakeForceStopLink(region reapable.Region, id reapable.ID, tokenSecret, apiURL string) (string, error) {
	stop, err := token.Tokenize(tokenSecret,
		token.NewForceStopJob(region.String(), id.String()))
	if err != nil {
		log.Error(fmt.Sprintf("Error creating ScaleToZero link: %s", err))
		return "", err
	}

	return makeURL(apiURL, "stop", stop), nil
}

func makeURL(host, action, token string) string {
	if host == "" {
		log.Critical("makeURL: host is empty")
	}

	action = url.QueryEscape(action)
	token = url.QueryEscape(token)

	vals := url.Values{}
	vals.Add(config.HTTP.Action, action)
	vals.Add(config.HTTP.Token, token)

	if host[len(host)-1:] == "/" {
		return host + "?" + vals.Encode()
	}
	return host + "/?" + vals.Encode()
}
