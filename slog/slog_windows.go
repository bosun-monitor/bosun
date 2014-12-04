package slog

import "code.google.com/p/winsvc/debug"

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
	e.Error(v)
}

func (e *eventLog) Info(v string) {
	e.l.Info(e.id, v)
}

func (e *eventLog) Warning(v string) {
	e.l.Warning(e.id, v)
}
func (e *eventLog) Error(v string) {
	e.l.Error(e.id, v)
}
