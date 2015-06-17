package aws

import (
	"fmt"
	"net/url"
	"time"

	"github.com/milescrabill/reaper/reapable"
	log "github.com/milescrabill/reaper/reaperlog"
	"github.com/milescrabill/reaper/token"
)

func MakeTerminateLink(region reapable.Region, id reapable.ID, tokenSecret, apiURL string) (string, error) {
	term, err := token.Tokenize(tokenSecret,
		token.NewTerminateJob(string(region), string(id)))

	if err != nil {
		return "", err
	}

	return makeURL(apiURL, "terminate", term), nil
}

func MakeIgnoreLink(region reapable.Region, id reapable.ID, tokenSecret, apiURL string,
	duration time.Duration) (string, error) {
	delay, err := token.Tokenize(tokenSecret,
		token.NewDelayJob(string(region), string(id),
			duration))

	if err != nil {
		return "", err
	}

	action := "delay_" + duration.String()
	return makeURL(apiURL, action, delay), nil

}

func MakeWhitelistLink(region reapable.Region, id reapable.ID, tokenSecret, apiURL string) (string, error) {
	whitelist, err := token.Tokenize(tokenSecret,
		token.NewWhitelistJob(string(region), string(id)))
	if err != nil {
		log.Error(fmt.Sprintf("Error creating whitelist link: %s", err))
		return "", err
	}

	return makeURL(apiURL, "whitelist", whitelist), nil
}

func MakeStopLink(region reapable.Region, id reapable.ID, tokenSecret, apiURL string) (string, error) {
	stop, err := token.Tokenize(tokenSecret,
		token.NewStopJob(string(region), string(id)))
	if err != nil {
		log.Error(fmt.Sprintf("Error creating ScaleToZero link: %s", err))
		return "", err
	}

	return makeURL(apiURL, "stop", stop), nil
}

func MakeForceStopLink(region reapable.Region, id reapable.ID, tokenSecret, apiURL string) (string, error) {
	stop, err := token.Tokenize(tokenSecret,
		token.NewForceStopJob(string(region), string(id)))
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
	} else {
		return host + "/?" + vals.Encode()
	}
}
