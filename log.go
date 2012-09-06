package main

import (
	"fmt"
	"log/syslog"
)

type Logger struct {
	*syslog.Writer
}

var logr *Logger

func NewLogger() (*Logger, error) {
	writer, err := syslog.New(syslog.LOG_EMERG, "cares")
	if err != nil {
		return nil, err
	}
	return &Logger{writer}, nil
}

func (l *Logger) Debugln(v ...interface{}) error {
	return l.Debug(fmt.Sprintln(v...))
}

func (l *Logger) Errln(v ...interface{}) error {
	return l.Err(fmt.Sprintln(v...))
}

func SetUpLogger() (err error) {
	logr, err = NewLogger()
	return
}
