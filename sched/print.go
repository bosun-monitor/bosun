package sched

import (
	"bytes"
	"log"

	"github.com/StackExchange/tsaf/conf"
)

func (s *Schedule) Print(a *conf.Alert, n *conf.Notification, st *State) {
	buf := new(bytes.Buffer)
	err := s.ExecuteSubject(buf, a, st)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(buf.String())
}
