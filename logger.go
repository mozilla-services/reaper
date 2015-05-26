package reaper

import (
	"log"
	"os"
)

// define a global package logger that goes to stdout
// and makes it easy to parse the output data
//
// format: date time what level ...data

var (
	_log = log.New(os.Stdout, "", log.Ldate|log.Ltime)
	Log  = &Logger{"-"} // the default logger
)

type Logger struct {
	What string
}

func out(what string, level string, v []interface{}) {
	x := append([]interface{}{what, level}, v...)
	_log.Println(x...)
}

func (l *Logger) Error(v ...interface{})   { out(l.What, "Error", v) }
func (l *Logger) Info(v ...interface{})    { out(l.What, "Info", v) }
func (l *Logger) Warning(v ...interface{}) { out(l.What, "Warn", v) }
