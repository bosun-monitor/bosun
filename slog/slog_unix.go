// +build !windows,!nacl,!plan9

package slog

import (
	"log"
	"log/syslog"
)

func SetSyslog() error {
	l, err := syslog.NewLogger(syslog.LOG_NOTICE, log.LstdFlags)
	if err != nil {
		return err
	}
	Set(&StdLog{Log: l})
	return nil
}
