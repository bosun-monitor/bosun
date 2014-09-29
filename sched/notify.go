package sched

import (
	"bytes"
	"log"
	"time"

	"github.com/StackExchange/bosun/conf"
	"github.com/StackExchange/bosun/expr"
)

// Poll dispatches notification checks when needed.
func (s *Schedule) Poll() {
	var timeout time.Duration = time.Hour
	for {
		// Wait for one of these two.
		select {
		case <-time.After(timeout):
		case <-s.nc:
		}
		timeout = s.CheckNotifications()
		s.Save()
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
func (s *Schedule) CheckNotifications() time.Duration {
	s.Lock()
	defer s.Unlock()
	timeout := time.Hour
	notifications := s.Notifications
	s.Notifications = nil
	for ak, ns := range notifications {
		for name, t := range ns {
			n, present := s.Conf.Notifications[name]
			if !present {
				continue
			}
			remaining := t.Add(n.Timeout).Sub(time.Now())
			if remaining > 0 {
				if remaining < timeout {
					timeout = remaining
				}
				s.AddNotification(ak, n, t)
				continue
			}
			st := s.status[ak]
			if st == nil {
				continue
			}
			s.Notify(st, n)
			if n.Timeout < timeout {
				timeout = n.Timeout
			}
		}
	}
	s.sendNotifications()
	s.notifications = nil
	return timeout
}

func (s *Schedule) sendNotifications() {
	for n, states := range s.notifications {
		ustates := make(States)
		for _, st := range states {
			if st.Last().Status == StUnknown {
				ustates[st.AlertKey()] = st
			} else {
				s.notify(st, n)
			}
			if n.Next != nil {
				s.AddNotification(st.AlertKey(), n, time.Now().UTC())
			}
		}
		for name, group := range ustates.GroupSets() {
			s.unotify(name, group, n)
		}
	}
}

func (s *Schedule) notify(st *State, n *conf.Notification) {
	a := s.Conf.Alerts[st.Alert]
	subject := new(bytes.Buffer)
	if err := s.ExecuteSubject(subject, a, st); err != nil {
		log.Println(err)
	}
	body := new(bytes.Buffer)
	c, err := s.ExecuteBody(body, a, st, true)
	if err != nil {
		log.Println(err)
		return
	}
	n.Notify(subject.Bytes(), body.Bytes(), s.Conf.EmailFrom, s.Conf.SmtpHost, st.Alert+st.Group.String(), c.Attachments...)
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
	n.Notify(subject.Bytes(), body.Bytes(), s.Conf.EmailFrom, name, s.Conf.SmtpHost)
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
