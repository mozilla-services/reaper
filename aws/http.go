package aws

import (
	"fmt"
	"net/url"
	"time"

	"github.com/mostlygeek/reaper/token"
)

func MakeTerminateLink(tokenSecret, apiURL, region, id string) (string, error) {
	term, err := token.Tokenize(tokenSecret,
		token.NewTerminateJob(region, id))

	if err != nil {
		return "", err
	}

	return makeURL(apiURL, "terminate", term), nil
}

func MakeIgnoreLink(tokenSecret, apiURL, region, id string,
	duration time.Duration) (string, error) {
	delay, err := token.Tokenize(tokenSecret,
		token.NewDelayJob(region, id,
			duration))

	if err != nil {
		return "", err
	}

	action := "delay_" + duration.String()
	return makeURL(apiURL, action, delay), nil

}

func MakeWhitelistLink(tokenSecret, apiURL, region, id string) (string, error) {
	whitelist, err := token.Tokenize(tokenSecret,
		token.NewWhitelistJob(region, id))
	if err != nil {
		log.Error(fmt.Sprintf("Error creating whitelist link: %s", err))
		return "", err
	}

	return makeURL(apiURL, "whitelist", whitelist), nil
}

func MakeStopLink(tokenSecret, apiURL, region, id string) (string, error) {
	stop, err := token.Tokenize(tokenSecret,
		token.NewStopJob(region, id))
	if err != nil {
		log.Error(fmt.Sprintf("Error creating ScaleToZero link: %s", err))
		return "", err
	}

	return makeURL(apiURL, "stop", stop), nil
}

func MakeForceStopLink(tokenSecret, apiURL, region, id string) (string, error) {
	stop, err := token.Tokenize(tokenSecret,
		token.NewForceStopJob(region, id))
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
