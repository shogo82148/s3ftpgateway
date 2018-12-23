package ftptest

import (
	"fmt"
	"testing"
)

type testLogger struct {
	t *testing.T
}

func (l testLogger) Print(sessionID string, message interface{}) {
	l.t.Helper()
	l.t.Logf("%s  %s", sessionID, message)
}

func (l testLogger) Printf(sessionID string, format string, v ...interface{}) {
	l.t.Helper()
	l.t.Log(sessionID, fmt.Sprintf(format, v...))
}

func (l testLogger) PrintCommand(sessionID string, command string, params string) {
	l.t.Helper()
	l.t.Logf("%s > %s %s", sessionID, command, params)
}

func (l testLogger) PrintResponse(sessionID string, code int, message string) {
	l.t.Helper()
	l.t.Logf("%s < %d %s", sessionID, code, message)
}
