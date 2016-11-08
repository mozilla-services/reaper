package http

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"

	reaperevents "github.com/mozilla-services/reaper/events"
	"github.com/mozilla-services/reaper/reapable"
	log "github.com/mozilla-services/reaper/reaperlog"
	"github.com/mozilla-services/reaper/state"
	"github.com/mozilla-services/reaper/token"
)

var versionString string

// Config is the configuration for the HTTP server
type Config struct {
	VersionFile string
	TokenSecret string
	APIURL      string
	Listen      string
	Token       string
	Action      string
}

type httpAPI struct {
	config Config
}

// Serve should be run in a goroutine
func (h *httpAPI) Serve() (e error) {
	// get version file
	bs, err := ioutil.ReadFile(h.config.VersionFile)
	if err != nil {
		log.Error("Could not open version.json: %s", err.Error())
		return
	}
	versionString = string(bs)

	router := mux.NewRouter()
	router.HandleFunc("/", processToken(h))
	router.HandleFunc("/__heartbeat__", heartbeat(h))
	router.HandleFunc("/__lbheartbeat__", heartbeat(h))
	router.HandleFunc("/__version__", version(h))

	log.Debug("Starting HTTP server: %s", h.config.Listen)
	go http.ListenAndServe(h.config.Listen, router)
	return nil
}

func NewAPI(c Config) *httpAPI {
	return &httpAPI{config: c}
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

func heartbeat(h *httpAPI) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		writeResponse(w, http.StatusOK, "Heart's a beatin'")
		return
	}
}

func version(h *httpAPI) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		writeResponse(w, http.StatusOK, versionString)
		return
	}
}

func processToken(h *httpAPI) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if err := req.ParseForm(); err != nil {
			writeResponse(w, http.StatusBadRequest, "Bad query string")
			return
		}

		userToken := req.Form.Get(h.config.Token)
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

		job, err := token.Untokenize(h.config.TokenSecret, userToken)
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
		r, err := reapable.Get(reapable.Region(job.Region), reapable.ID(job.ID))
		if err != nil {
			writeResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		switch job.Action {
		case token.J_DELAY:
			log.Debug("Delay request received for %s in region %s until %s",
				job.ID,
				job.Region,
				job.IgnoreUntil.String())
			s := r.ReaperState()
			ok, err := r.Save(state.NewStateWithUntilAndState(s.Until.Add(job.IgnoreUntil), s.State))
			if err != nil {
				writeResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !ok {
				writeResponse(w, http.StatusInternalServerError,
					fmt.Sprintf("Delay failed for %s.", r.ReapableDescriptionTiny()))
				return
			}
			reaperevents.NewEvent("Reaper: Delay Request Received",
				fmt.Sprintf("Delay for %s in region %s until %s",
					job.ID,
					job.Region,
					job.IgnoreUntil.String()),
				nil,
				[]string{},
			)
			reaperevents.NewCountStatistic("reaper.reapables.requests", []string{"type:delay"})
		case token.J_TERMINATE:
			log.Debug("Terminate request received for %s in region %s.", job.ID, job.Region)
			ok, err := r.Terminate()
			if err != nil {
				writeResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !ok {
				writeResponse(w, http.StatusInternalServerError,
					fmt.Sprintf("Terminate failed for %s.", r.ReapableDescriptionTiny()))
				return
			}
			reaperevents.NewEvent("Reaper: Terminate Request Received",
				r.ReapableDescriptionShort(), nil, []string{})
			reaperevents.NewCountStatistic("reaper.reapables.requests",
				[]string{"type:terminate"})
		case token.J_WHITELIST:
			log.Debug("Whitelist request received for %s in region %s", job.ID, job.Region)
			ok, err := r.Whitelist()
			if err != nil {
				writeResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !ok {
				writeResponse(w, http.StatusInternalServerError,
					fmt.Sprintf("Whitelist failed for %s.", r.ReapableDescriptionTiny()))
				return
			}
			reaperevents.NewEvent("Reaper: Whitelist Request Received",
				r.ReapableDescriptionShort(), nil, []string{})
			reaperevents.NewCountStatistic("reaper.reapables.requests",
				[]string{"type:whitelist"})
		case token.J_STOP:
			log.Debug("Stop request received for %s in region %s", job.ID, job.Region)
			ok, err := r.Stop()
			if err != nil {
				writeResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !ok {
				writeResponse(w, http.StatusInternalServerError,
					fmt.Sprintf("Stop failed for %s.", r.ReapableDescriptionTiny()))
				return
			}
			reaperevents.NewEvent("Reaper: Stop Request Received",
				r.ReapableDescriptionShort(), nil, []string{})
			reaperevents.NewCountStatistic("reaper.reapables.requests", []string{"type:stop"})
		default:
			log.Error("Unrecognized job token received.")
			writeResponse(w, http.StatusInternalServerError, "Unrecognized job token.")
			return
		}

		writeResponse(w, http.StatusOK, fmt.Sprintf("Success. Check %s out.", r.ReapableDescriptionTiny()))
	}
}
