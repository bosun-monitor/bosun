package sched

import (
	"bytes"
	"log"
	"net/http"

	"github.com/StackExchange/tsaf/conf"
)

func (s *Schedule) Post(a *conf.Alert, n *conf.Notification, st *State) {
	buf := new(bytes.Buffer)
	err := s.ExecuteSubject(buf, a, st)
	if err != nil {
		log.Println(err)
		return
	}
	resp, err := http.Post(n.Post.String(), "application/x-www-form-urlencoded", buf)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		log.Println(err)
		return
	}
	if resp.StatusCode >= 300 {
		log.Println("bad response on notification post:", resp.Status)
	}
}

func (s *Schedule) Get(a *conf.Alert, n *conf.Notification, st *State) {
	resp, err := http.Get(n.Get.String())
	if err != nil {
		log.Println(err)
		return
	}
	if resp.StatusCode >= 300 {
		log.Println("bad response on notification get:", resp.Status)
	}
}
