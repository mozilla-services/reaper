package reaper

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/mostlygeek/reaper/token"
	. "github.com/tj/go-debug"
)

var (
	debugHTTP = Debug("reaper:http")
)

const (
	HTTP_TOKEN_VAR  = "t"
	HTTP_ACTION_VAR = "a"
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

	debugHTTP("Starting HTTP server: %s", h.conf.HTTPListen)
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

		userToken := req.Form.Get(HTTP_TOKEN_VAR)
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

		if job.Action == token.J_DELAY {
			debugHTTP("Delay %s in %s until %s", job.InstanceId, job.Region, job.IgnoreUntil.String())
			err := UpdateReaperState(job.Region, job.InstanceId, &State{
				State: STATE_IGNORE,
				Until: job.IgnoreUntil,
			})

			if err != nil {
				writeResponse(w, http.StatusInternalServerError, err.Error())
				return
			}

		} else if job.Action == token.J_TERMINATE {
			debugHTTP("Terminate %s", job.InstanceId)
			err := Terminate(job.Region, job.InstanceId)
			if err != nil {
				writeResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		writeResponse(w, http.StatusOK, "OK")
	}
}

func MakeTerminateLink(tokenSecret, apiUrl, region, id string) (string, error) {
	term, err := token.Tokenize(tokenSecret,
		token.NewTerminateJob(region, id))

	if err != nil {
		return "", err
	}

	return makeURL(apiUrl, "terminate", term), nil
}

func MakeIgnoreLink(tokenSecret, apiUrl, region, id string,
	duration time.Duration) (string, error) {
	delay, err := token.Tokenize(tokenSecret,
		token.NewDelayJob(region, id,
			time.Now().Add(duration)))

	if err != nil {
		return "", err
	}

	action := "delay_" + duration.String()
	return makeURL(apiUrl, action, delay), nil

}

func makeURL(host, action, token string) string {
	action = url.QueryEscape(action)
	token = url.QueryEscape(token)

	vals := url.Values{}
	vals.Add(HTTP_ACTION_VAR, action)
	vals.Add(HTTP_TOKEN_VAR, token)

	if host[len(host)-1:] == "/" {
		return host + "?" + vals.Encode()
	} else {
		return host + "/?" + vals.Encode()
	}
}
