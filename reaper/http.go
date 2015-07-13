package reaper

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	reaperaws "github.com/mozilla-services/reaper/aws"
	reaperevents "github.com/mozilla-services/reaper/events"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
	"github.com/mozilla-services/reaper/token"
)

type HTTPApi struct {
	conf   reaperevents.HTTPConfig
	server *http.Server
	ln     net.Listener
}

// Serve should be run in a goroutine
func (h *HTTPApi) Serve() (e error) {
	h.ln, e = net.Listen("tcp", h.conf.Listen)

	if e != nil {
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", processToken(h))
	mux.HandleFunc("/__heartbeat__", heartbeat(h))
	h.server = &http.Server{Handler: mux}

	log.Debug("Starting HTTP server: %s", h.conf.Listen)
	go h.server.Serve(h.ln)
	return nil
}

// Stop will close the listener, it waits for nothing
func (h *HTTPApi) Stop() (e error) {
	return h.ln.Close()
}

func NewHTTPApi(c reaperevents.HTTPConfig) *HTTPApi {
	return &HTTPApi{conf: c}
}

func writeResponse(w http.ResponseWriter, code int, body string) {
	w.Header().Set("Content-Type", "text/html")
	io.WriteString(w, fmt.Sprintf(`<DOCTYPE html>
		<html>
			<head>
				<title>Reaper API</title>
			</head>
			<body>
				<p>
				%s
				</p>
			</body>
		</html>`,
		body))
}

func heartbeat(h *HTTPApi) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		writeResponse(w, http.StatusOK, "Heart's a beatin'")
		return
	}
}

func processToken(h *HTTPApi) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if err := req.ParseForm(); err != nil {
			writeResponse(w, http.StatusBadRequest, "Bad query string")
			return
		}

		userToken := req.Form.Get(h.conf.Token)
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
		r, err := reapables.Get(reapable.Region(job.Region), reapable.ID(job.ID))
		if err != nil {
			writeResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		switch job.Action {
		case token.J_DELAY:
			log.Debug("Delay request received for %s in region %s until %s", job.ID, job.Region, job.IgnoreUntil.String())
			s := r.ReaperState()
			ok, err := r.Save(
				state.NewStateWithUntilAndState(s.Until.Add(job.IgnoreUntil), s.State))
			if err != nil {
				writeResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !ok {
				writeResponse(w, http.StatusInternalServerError, fmt.Sprintf("Delay failed for %s.", r.ReapableDescriptionTiny()))
				return
			}
		case token.J_TERMINATE:
			log.Debug("Terminate request received for %s in region %s.", job.ID, job.Region)
			ok, err := r.Terminate()
			if err != nil {
				writeResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !ok {
				writeResponse(w, http.StatusInternalServerError, fmt.Sprintf("Terminate failed for %s.", r.ReapableDescriptionTiny()))
				return
			}
		case token.J_WHITELIST:
			log.Debug("Whitelist request received for %s in region %s", job.ID, job.Region)
			ok, err := r.Whitelist()
			if err != nil {
				writeResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !ok {
				writeResponse(w, http.StatusInternalServerError, fmt.Sprintf("Whitelist failed for %s.", r.ReapableDescriptionTiny()))
				return
			}
		case token.J_STOP:
			log.Debug("Stop request received for %s in region %s", job.ID, job.Region)
			ok, err := r.Stop()
			if err != nil {
				writeResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !ok {
				writeResponse(w, http.StatusInternalServerError, fmt.Sprintf("Stop failed for %s.", r.ReapableDescriptionTiny()))
				return
			}
		case token.J_FORCESTOP:
			log.Debug("Force Stop request received for %s in region %s", job.ID, job.Region)
			ok, err := r.ForceStop()
			if err != nil {
				writeResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !ok {
				writeResponse(w, http.StatusInternalServerError, fmt.Sprintf("ForceStop failed for %s.", r.ReapableDescriptionTiny()))
				return
			}
		default:
			log.Error("Unrecognized job token received.")
			writeResponse(w, http.StatusInternalServerError, "Unrecognized job token.")
			return
		}

		var consoleURL *url.URL
		switch t := r.(type) {
		case *reaperaws.Instance:
			consoleURL = t.AWSConsoleURL()
		case *reaperaws.AutoScalingGroup:
			consoleURL = t.AWSConsoleURL()
		default:
			log.Error("No AWSConsoleURL")
		}
		writeResponse(w, http.StatusOK, fmt.Sprintf("Success. Check %s out on the <a href=\"%s\">AWS Console.</a>", r.ReapableDescriptionTiny(), consoleURL))
	}
}
