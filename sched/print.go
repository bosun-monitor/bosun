package sched

import (
	"bytes"
	"log"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/conf"
)

func (s *Schedule) Print(a *conf.Alert, n *conf.Notification, group opentsdb.TagSet) {
	buf := new(bytes.Buffer)
	err := a.ExecuteSubject(buf, group, s.cache)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(buf.String())
}
