package sched

import (
	"bytes"
	"log"
	"time"

	"github.com/StackExchange/tsaf/conf"
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
			st, present := s.Status[ak]
			if !present {
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
		var ustates []*State
		for _, st := range states {
			if st.Last().Status == stUnknown {
				ustates = append(ustates, st)
			} else {
				s.notify(st, n)
			}
			if n.Next != nil {
				s.AddNotification(st.AlertKey(), n, time.Now().UTC())
			}
		}
		for _, group := range GroupSets(ustates) {
			s.unotify(group, n)
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
	if err := s.ExecuteBody(body, a, st); err != nil {
		log.Println(err)
	}
	n.Notify(subject.Bytes(), body.Bytes(), s.Conf.EmailFrom, s.Conf.SmtpHost)
}

func (s *Schedule) unotify(group AlertKeys, n *conf.Notification) {
	subject := new(bytes.Buffer)
	body := new(bytes.Buffer)
	now := time.Now().UTC()
	s.Group[now] = group
	if t := s.Conf.UnknownTemplate; t != nil {
		data := s.unknownData(now, group)
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
	n.Notify(subject.Bytes(), body.Bytes(), s.Conf.EmailFrom, s.Conf.SmtpHost)
}

func (s *Schedule) AddNotification(ak AlertKey, n *conf.Notification, started time.Time) {
	if s.Notifications == nil {
		s.Notifications = make(map[AlertKey]map[string]time.Time)
	}
	if s.Notifications[ak] == nil {
		s.Notifications[ak] = make(map[string]time.Time)
	}
	s.Notifications[ak][n.Name] = started
}

// GroupSets returns slices of TagSets, grouped by most common ancestor. Empty
// TagSets are grouped together.
func GroupSets(states []*State) []AlertKeys {
	type Pair struct {
		k, v string
	}
	var groups []AlertKeys
	seen := make(map[*State]bool)
	for {
		counts := make(map[Pair]int)
		for _, s := range states {
			if seen[s] {
				continue
			}
			for k, v := range s.Group {
				counts[Pair{k, v}]++
			}
		}
		if len(counts) == 0 {
			break
		}
		max := 0
		var pair Pair
		for p, c := range counts {
			if c > max {
				max = c
				pair = p
			}
		}
		var group AlertKeys
		for _, s := range states {
			if seen[s] {
				continue
			}
			if s.Group[pair.k] != pair.v {
				continue
			}
			seen[s] = true
			group = append(group, s.AlertKey())
		}
		if len(group) > 0 {
			groups = append(groups, group)
		}
	}
	// empties
	var group AlertKeys
	for _, s := range states {
		if seen[s] {
			continue
		}
		group = append(group, s.AlertKey())
	}
	if len(group) > 0 {
		groups = append(groups, group)
	}
	return groups
}
