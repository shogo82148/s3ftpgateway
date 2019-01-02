package main

import (
	"github.com/sirupsen/logrus"
)

type logger struct{}

func (logger) Print(sessionID string, message interface{}) {
	logrus.WithField("session_id", sessionID).Info(message)
}

func (logger) Printf(sessionID string, format string, v ...interface{}) {
	logrus.WithField("session_id", sessionID).Infof(format, v...)
}

func (logger) PrintCommand(sessionID string, command string, params string) {
	logrus.WithFields(logrus.Fields{
		"session_id": sessionID,
		"command":    command,
		"params":     params,
	}).Info("command")
}

func (logger) PrintResponse(sessionID string, code int, message string) {
	logrus.WithFields(logrus.Fields{
		"session_id": sessionID,
		"code":       code,
		"message":    message,
	}).Info("response")
}
