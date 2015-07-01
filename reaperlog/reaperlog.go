package reaperlog

import (
	"fmt"
	"os"

	"github.com/op/go-logging"
)

var log *logging.Logger
var config LogConfig

type LogConfig struct {
	Extras bool
}

func EnableExtras() {
	config.Extras = true
}

func Extras() bool {
	return config.Extras
}

func init() {
	// set up logging
	log = logging.MustGetLogger("Reaper")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	format := logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} ▶%{color:reset} %{message}")
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)
}

func AddLogFile(filename string) {
	if filename != "" {
		// open file write only, append mode
		// create it if it doesn't exist
		f, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
		if err != nil {
			log.Error(fmt.Sprintf("Unable to open logfile '%s'", filename))
		} else {
			// if the file was successfully opened
			log.Info("Logging to %s", filename)
			// reconfigure logging with stdout and logfile as outputs
			logFileFormat := logging.MustStringFormatter("%{time:15:04:05.000}: %{shortfunc} ▶ %{level:.4s} ▶ %{message}")
			logFileBackend := logging.NewLogBackend(f, "", 0)
			logFileBackendFormatter := logging.NewBackendFormatter(logFileBackend, logFileFormat)

			backend := logging.NewLogBackend(os.Stderr, "", 0)
			format := logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} ▶%{color:reset} %{message}")
			backendFormatter := logging.NewBackendFormatter(backend, format)
			logging.SetBackend(backendFormatter, logFileBackendFormatter)
		}
	}
}

func Debug(format string, args ...interface{}) {
	log.Debug(format, args...)
}

func Info(format string, args ...interface{}) {
	log.Info(format, args...)
}

func Warning(format string, args ...interface{}) {
	log.Warning(format, args...)
}

func Critical(format string, args ...interface{}) {
	log.Critical(format, args...)
}

func Fatal(args ...interface{}) {
	log.Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	log.Panicf(format, args...)
}

func Panic(args ...interface{}) {
	log.Panic(args...)
}

func Panicf(format string, args ...interface{}) {
	log.Panicf(format, args...)
}

func Error(format string, args ...interface{}) {
	log.Error(format, args...)
}

func Notice(format string, args ...interface{}) {
	log.Notice(format, args...)
}
