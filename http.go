package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/mostlygeek/reaper/token"
)

const (
	httpToken  = "t"
	httpAction = "a"
)

type HTTPApi struct {
	conf   Config
	server *http.Server
	ln     net.Listener
}

// Serve should be run in a goroutine
func (h *HTTPApi) Serve() (e error) {
	h.ln, e = net.Listen("tcp", h.conf.HTTPListen)

	if e != nil {
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", processToken(h))
	h.server = &http.Server{Handler: mux}

	Log.Debug("Starting HTTP server: %s", h.conf.HTTPListen)
	go h.server.Serve(h.ln)
	return nil
}

// Stop will close the listener, it waits for nothing
func (h *HTTPApi) Stop() (e error) {
	return h.ln.Close()
}

func NewHTTPApi(c Config) *HTTPApi {
	return &HTTPApi{conf: c}
}

func writeResponse(w http.ResponseWriter, code int, body string) {
	w.WriteHeader(code)
	io.WriteString(w, body)
}

func processToken(h *HTTPApi) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if err := req.ParseForm(); err != nil {
			writeResponse(w, http.StatusBadRequest, "Bad query string")
			return
		}

		userToken := req.Form.Get(httpToken)
		if userToken == "" {
			writeResponse(w, http.StatusBadRequest, "Token Missing")
			return
		}

		if u, err := url.QueryUnescape(userToken); err == nil {
			userToken = u
		} else {
			writeResponse(w,
				http.StatusBadRequest, "Invalid Token, could not decode data")
			return
		}

		job, err := token.Untokenize(h.conf.TokenSecret, userToken)
		if err != nil {
			writeResponse(w,
				http.StatusBadRequest, "Invalid Token, Could not untokenize")
			return
		}

		if job.Expired() == true {
			writeResponse(w, http.StatusBadRequest, "Token expired")
			return
		}

		// find reapable associated with the job
		r, ok := Reapables[job.Region][job.ID]

		// reapable not found
		if !ok {
			writeResponse(w, http.StatusInternalServerError, fmt.Sprintf("Reapable %s in region %s not found.", job.ID, job.Region))
			return
		}

		switch job.Action {
		case token.J_DELAY:
			Log.Debug("Delay request received for %s in region %s until %s", job.ID, job.Region, job.IgnoreUntil.String())
			state := r.ReaperState()
			_, err := r.Save(&State{
				State: state.State,
				Until: state.Until.Add(job.IgnoreUntil),
			})
			if err != nil {
				writeResponse(w, http.StatusInternalServerError, err.Error())
				return
			}

		case token.J_TERMINATE:
			Log.Debug("Terminate request received for %s in region %s.", job.ID, job.Region)
			_, err := r.Terminate()
			if err != nil {
				writeResponse(w, http.StatusInternalServerError, err.Error())
				return
			}

		case token.J_WHITELIST:
			Log.Debug("Whitelist request received for %s in region %s", job.ID, job.Region)
			_, err := r.Whitelist()
			if err != nil {
				writeResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
		case token.J_STOP:
			Log.Debug("Stop request received for %s in region %s", job.ID, job.Region)
			_, err := r.Stop()
			if err != nil {
				writeResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
		case token.J_FORCESTOP:
			Log.Debug("Force Stop request received for %s in region %s", job.ID, job.Region)
			_, err := r.ForceStop()
			if err != nil {
				writeResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
		default:
			Log.Error("Unrecognized job token received.")
			writeResponse(w, http.StatusInternalServerError, "Unrecognized job token.")
			return
		}
		writeResponse(w, http.StatusOK, fmt.Sprintf("Resource state: %s", r.ReaperState().String()))
	}
}

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
		Log.Error("Error creating whitelist link: %s", err)
		return "", err
	}

	return makeURL(apiURL, "whitelist", whitelist), nil
}

func MakeStopLink(tokenSecret, apiURL, region, id string) (string, error) {
	stop, err := token.Tokenize(tokenSecret,
		token.NewStopJob(region, id))
	if err != nil {
		Log.Error("Error creating ScaleToZero link: %s", err)
		return "", err
	}

	return makeURL(apiURL, "stop", stop), nil
}

func MakeForceStopLink(tokenSecret, apiURL, region, id string) (string, error) {
	stop, err := token.Tokenize(tokenSecret,
		token.NewForceStopJob(region, id))
	if err != nil {
		Log.Error("Error creating ScaleToZero link: %s", err)
		return "", err
	}

	return makeURL(apiURL, "stop", stop), nil
}

func makeURL(host, action, token string) string {
	action = url.QueryEscape(action)
	token = url.QueryEscape(token)

	vals := url.Values{}
	vals.Add(httpAction, action)
	vals.Add(httpToken, token)

	if host[len(host)-1:] == "/" {
		return host + "?" + vals.Encode()
	} else {
		return host + "/?" + vals.Encode()
	}
}
