package ftp

import (
	"fmt"
	"log"
	"testing"
)

type testLogger struct {
	t *testing.T
}

func (l testLogger) Print(sessionID string, message interface{}) {
	l.t.Logf("%s  %s", sessionID, message)
}

func (l testLogger) Printf(sessionID string, format string, v ...interface{}) {
	l.t.Log(sessionID, fmt.Sprintf(format, v...))
}

func (l testLogger) PrintCommand(sessionID string, command string, params string) {
	l.t.Logf("%s > %s %s", sessionID, command, params)
}

func (l testLogger) PrintResponse(sessionID string, code int, message string) {
	log.Printf("%s < %d %s", sessionID, code, message)
}
