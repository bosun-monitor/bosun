package sched

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/expr"
)

// Poll dispatches notification checks when needed.
func (s *Schedule) Poll() {
	for {
		rh := s.NewRunHistory(time.Now())
		timeout := s.CheckNotifications(rh)
		s.Save()
		// Wait for one of these two.
		select {
		case <-time.After(timeout):
		case <-s.nc:
		}
	}
}

func (s *Schedule) Notify(st *State, n *conf.Notification) {
	if s.notifications == nil {
		s.notifications = make(map[*conf.Notification][]*State)
	}
	s.notifications[n] = append(s.notifications[n], st)
}

// CheckNotifications processes past notification events. It returns the
// duration until the soonest notification triggers.
func (s *Schedule) CheckNotifications(rh *RunHistory) time.Duration {
	silenced := s.Silenced()
	s.Lock()
	defer s.Unlock()
	notifications := s.Notifications
	s.Notifications = nil
	for ak, ns := range notifications {
		if _, present := silenced[ak]; present {
			log.Println("silencing", ak)
			continue
		}
		for name, t := range ns {
			n, present := s.Conf.Notifications[name]
			if !present {
				continue
			}
			remaining := t.Add(n.Timeout).Sub(time.Now())
			if remaining > 0 {
				s.AddNotification(ak, n, t)
				continue
			}
			st := s.status[ak]
			if st == nil {
				continue
			}
			s.Notify(st, n)
		}
	}
	s.sendNotifications(rh, silenced)
	s.notifications = nil
	timeout := time.Hour
	now := time.Now()
	for _, ns := range s.Notifications {
		for name, t := range ns {
			n, present := s.Conf.Notifications[name]
			if !present {
				continue
			}
			remaining := t.Add(n.Timeout).Sub(now)
			if remaining < timeout {
				timeout = remaining
			}
		}
	}
	return timeout
}

func (s *Schedule) sendNotifications(rh *RunHistory, silenced map[expr.AlertKey]Silence) {
	if s.Conf.Quiet {
		log.Println("quiet mode prevented", len(s.notifications), "notifications")
		return
	}
	for n, states := range s.notifications {
		ustates := make(States)
		for _, st := range states {
			ak := st.AlertKey()
			if st.Last().Status == StUnknown {
				if _, ok := silenced[ak]; ok {
					log.Println("silencing unknown", ak)
					continue
				}
				ustates[ak] = st
			} else {
				s.notify(rh, st, n)
			}
			if n.Next != nil {
				s.AddNotification(ak, n, time.Now().UTC())
			}
		}
		for name, group := range ustates.GroupSets() {
			s.unotify(name, group, n)
		}
	}
}

func (s *Schedule) notify(rh *RunHistory, st *State, n *conf.Notification) {
	a := s.Conf.Alerts[st.Alert]
	subject := new(bytes.Buffer)
	var s_err, b_err error
	if s_err = s.ExecuteSubject(subject, rh, a, st); s_err != nil {
		log.Printf("%s: %v", st.AlertKey(), s_err)
	}
	body := new(bytes.Buffer)
	attachments, b_err := s.ExecuteBody(body, rh, a, st, true)
	if b_err != nil {
		log.Printf("%s: %v", st.AlertKey(), b_err)
	}
	if s_err != nil || b_err != nil {
		var err error
		subject, body, err = s.ExecuteBadTemplate(s_err, b_err, rh, a, st)
		if err != nil {
			subject = bytes.NewBufferString(fmt.Sprintf("unable to create tempalate error notification: %v", err))
		}
	}
	n.Notify(subject.Bytes(), body.Bytes(), s.Conf, string(st.AlertKey()), attachments...)
}

func (s *Schedule) unotify(name string, group expr.AlertKeys, n *conf.Notification) {
	subject := new(bytes.Buffer)
	body := new(bytes.Buffer)
	now := time.Now().UTC()
	s.Group[now] = group
	if t := s.Conf.UnknownTemplate; t != nil {
		data := s.unknownData(now, name, group)
		if t.Body != nil {
			if err := t.Body.Execute(body, &data); err != nil {
				log.Println("unknown template error:", err)
			}
		}
		if t.Subject != nil {
			if err := t.Subject.Execute(subject, &data); err != nil {
				log.Println("unknown template error:", err)
			}
		}
	}
	n.Notify(subject.Bytes(), body.Bytes(), s.Conf, name)
}

func (s *Schedule) AddNotification(ak expr.AlertKey, n *conf.Notification, started time.Time) {
	if s.Notifications == nil {
		s.Notifications = make(map[expr.AlertKey]map[string]time.Time)
	}
	if s.Notifications[ak] == nil {
		s.Notifications[ak] = make(map[string]time.Time)
	}
	s.Notifications[ak][n.Name] = started
}
