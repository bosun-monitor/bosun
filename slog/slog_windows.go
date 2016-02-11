package slog

import (
	"fmt"

	"bosun.org/_third_party/golang.org/x/sys/windows/svc/debug"
)

type eventLog struct {
	l  debug.Log
	id uint32
}

// Sets the logger to a Windows Event Log. Designed for use with the
// code.google.com/p/winsvc/eventlog and code.google.com/p/winsvc/debug
// packages.
func SetEventLog(l debug.Log, eid uint32) {
	Set(&eventLog{l, eid})
}

func (e *eventLog) Fatal(v string) {
	e.l.Error(e.id, fmt.Sprintf("fatal: %s", v))
}

func (e *eventLog) Info(v string) {
	e.l.Info(e.id, fmt.Sprintf("info: %s", v))
}

func (e *eventLog) Warning(v string) {
	e.l.Warning(e.id, fmt.Sprintf("warning: %s", v))
}
func (e *eventLog) Error(v string) {
	e.l.Error(e.id, fmt.Sprintf("error: %s", v))
}

func (e *eventLog) Debug(v string) {
	// Windows logger doesn't have Debug, so use Info instead
	e.l.Info(e.id, fmt.Sprintf("debug: %s", v))
}
