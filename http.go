package reaper

import (
	"io"
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
	conf Config
}

// Serve should be run in a goroutine
func (h *HTTPApi) Serve() error {
	http.HandleFunc("/", processToken(h))
	debugHTTP("Starting HTTP server: %s", h.conf.HTTPListen)
	return http.ListenAndServe(h.conf.HTTPListen, nil)
}

func NewHTTPApi(c Config) *HTTPApi {
	return &HTTPApi{conf: c}
}

func processToken(h *HTTPApi) func(http.ResponseWriter, *http.Request) {

	return func(w http.ResponseWriter, req *http.Request) {

		if err := req.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "Bad query string")
			return
		}

		userToken := req.Form.Get(HTTP_TOKEN_VAR)
		if userToken == "" {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "Token Missing\n")
			return
		}

		if u, err := url.QueryUnescape(userToken); err == nil {
			userToken = u
		} else {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "Invalid Token, could not decode data\n")
			return
		}

		job, err := token.Untokenize(h.conf.TokenSecret, userToken)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "Invalid Token, could not untokenize\n")
			return
		}

		if job.Expired() == true {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "Token expired\n")
			return
		}

		if job.Action == token.J_DELAY {
			debugHTTP("Delay %s until %s", job.InstanceId, job.IgnoreUntil.String())
			ec2
		} else if job.Action == token.J_TERMINATE {
			debugHTTP("Terminate %s", job.InstanceId)
			// terminate the process
		}
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

func MakeIgnoreLink(tokenSecret, apiUrl, region, id string, duration time.Duration) (string, error) {
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
