package reaperlog

import (
	log "github.com/Sirupsen/logrus"
	"github.com/rifflock/lfshook"
	"go.mozilla.org/mozlogrus"
)

var config LogConfig

type LogConfig struct {
	Extras bool
}

func EnableExtras() {
	config.Extras = true
}

func EnableMozlog() {
	mozlogrus.Enable("Reaper")
}

func Extras() bool {
	return config.Extras
}

func SetConfig(c *LogConfig) {
	config = *c
}

func AddLogFile(filename string) {
	log.AddHook(lfshook.NewHook(lfshook.PathMap{
		log.DebugLevel: filename,
		log.InfoLevel:  filename,
		log.WarnLevel:  filename,
		log.ErrorLevel: filename,
		log.FatalLevel: filename,
		log.PanicLevel: filename,
	}))
}

func Debug(format string, args ...interface{}) {
	log.Debugf(format, args...)
}

func Info(format string, args ...interface{}) {
	log.Infof(format, args...)
}

func Warning(format string, args ...interface{}) {
	log.Warningf(format, args...)
}

func Fatal(format string, args ...interface{}) {
	log.Fatalf(format, args...)
}

func Panic(format string, args ...interface{}) {
	log.Panicf(format, args...)
}

func Error(format string, args ...interface{}) {
	log.Errorf(format, args...)
}
