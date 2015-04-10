// +build !windows,!nacl,!plan9

package main

import (
	"flag"
	"log"
	"log/syslog"
)

var noSyslog = flag.Bool("disable-syslog", false, "disables logging to syslog")

func init() {
	mains = append(mains, setSyslog)
}

func setSyslog() {
	if *noSyslog || *flagDev || *flagTest {
		return
	}
	w, err := syslog.New(syslog.LOG_LOCAL6|syslog.LOG_INFO, "bosun")
	if err != nil {
		log.Printf("could not open syslog: %v", err)
		return
	}
	log.Println("enabling syslog")
	log.SetOutput(w)
}
