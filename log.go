package main

import (
	"log"
	"log/syslog"
	"os"
)

type Logger struct {
	*log.Logger
	level syslog.Priority
}

var logr *Logger

func NewLogger() (*Logger, error) {
	writer := os.Stderr
	logger := log.New(writer, log.Prefix(), log.LstdFlags)
	return &Logger{logger, syslog.LOG_DEBUG}, nil
}

func (l *Logger) Debugln(v ...interface{}) error {
	if l.level >= syslog.LOG_DEBUG {
		l.Println(v...)
	}
	return nil
}

func (l *Logger) Errln(v ...interface{}) error {
	if l.level >= syslog.LOG_ERR {
		l.Println(v...)
	}
	return nil
}

func (l *Logger) Close() error {
	return nil
}

func SetUpLogger() (err error) {
	logr, err = NewLogger()
	return
}
