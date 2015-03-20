package main

import (
	"io"
	"log"
	"net/http"

	"github.com/mostlygeek/reaper/token"
)

const (
	pass = "test"
)

func ProcessToken(w http.ResponseWriter, req *http.Request) {

	userToken := req.RequestURI[1:]
	job, err := token.Untokenize("test", userToken)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "Invalid Token\n")
		return
	}

	if job.Expired() == true {
		w.WriteHeader(http.StatusGone)
		io.WriteString(w, "Token expired\n")
		return
	}

	if job.Type == token.J_DELAY {
		// delay the process
	} else if job.Type == token.J_TERMINATE {
		// terminate the process
	}

}

func main() {
	http.HandleFunc("/", ProcessToken)

	exit := make(chan struct{})
	go func() {
		err := http.ListenAndServe(":9999", nil)
		if err != nil {
			log.Fatal("ListenAndServe:", err)
		}
	}()

	// just block until killed
	<-exit
}
