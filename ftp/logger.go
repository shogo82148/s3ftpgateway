package ftp

import (
	"fmt"
	"log"
)

// Logger is a logger for the ftp server.
type Logger interface {
	Print(sessionID string, message interface{})
	Printf(sessionID string, format string, v ...interface{})
	PrintCommand(sessionID string, command string, params string)
	PrintResponse(sessionID string, code int, message string)
}

// StdLogger logs uging the logger package.
var StdLogger Logger = stdLogger{}

type stdLogger struct{}

func (stdLogger) Print(sessionID string, message interface{}) {
	log.Printf("%s  %s", sessionID, message)
}

func (stdLogger) Printf(sessionID string, format string, v ...interface{}) {
	log.Print(sessionID, " ", fmt.Sprintf(format, v...))
}

func (stdLogger) PrintCommand(sessionID string, command string, params string) {
	log.Printf("%s > %s %s", sessionID, command, params)
}

func (stdLogger) PrintResponse(sessionID string, code int, message string) {
	log.Printf("%s < %d %s", sessionID, code, message)
}

// DiscardLogger discards all logs.
var DiscardLogger Logger = discardLogger{}

type discardLogger struct{}

func (discardLogger) Print(sessionID string, message interface{})                  {}
func (discardLogger) Printf(sessionID string, format string, v ...interface{})     {}
func (discardLogger) PrintCommand(sessionID string, command string, params string) {}
func (discardLogger) PrintResponse(sessionID string, code int, message string)     {}
