// +build !windows,!nacl,!plan9

package slog

import "log/syslog"

func SetSyslog() error {
	w, err := syslog.New(syslog.LOG_LOCAL6, "")
	if err != nil {
		return err
	}
	Set(&Syslog{W: w})
	return nil
}

type Syslog struct {
	W *syslog.Writer
}

func (s *Syslog) Fatal(v string) {
	s.W.Crit("crit: " + v)
}

func (s *Syslog) Error(v string) {
	s.W.Err("error: " + v)
}

func (s *Syslog) Info(v string) {
	// Mac OSX ignores levels info and debug by default, so use notice.
	s.W.Notice("info: " + v)
}

func (s *Syslog) Warning(v string) {
	s.W.Warning("warning: " + v)
}
