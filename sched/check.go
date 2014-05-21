package sched

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"github.com/StackExchange/tsaf/_third_party/github.com/StackExchange/scollector/collect"
	"github.com/StackExchange/tsaf/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/expr"
)

func (s *Schedule) Status(ak AlertKey) *State {
	s.Lock()
	st := s.status[ak]
	s.Unlock()
	return st
}

func (s *Schedule) Check() {
	checkNotify := s.RunChecks()
	if checkNotify {
		s.nc <- true
	}
	s.Save()
}

func (s *Schedule) RunChecks() bool {
	s.CheckStart = time.Now().UTC()
	s.RunHistory = make(map[AlertKey]Event)
	s.cache = opentsdb.NewCache(s.Conf.TsdbHost, s.Conf.ResponseLimit)
	for _, a := range s.Conf.Alerts {
		s.CheckAlert(a)
	}
	s.CheckUnknown()
	checkNotify := false
	silenced := s.Silenced()
	for ak, event := range s.RunHistory {
		state := s.Status(ak)
		// This got the last, and also put the current status onto history. So now I can populate
		// History lower in the stack
		last := state.Append(event)
		//last := state.Last().Status
		a := s.Conf.Alerts[ak.Name()]
		if event.Status > StNormal {
			var subject = new(bytes.Buffer)
			if event.Status != StUnknown {
				if err := s.ExecuteSubject(subject, a, state); err != nil {
					log.Println(err)
				}
			}
			state.Subject = subject.String()
			state.Open = true
		}
		// On state increase, clear old notifications and notify current.
		// On state decrease, and if the old alert was already acknowledged, notify current.
		// If the old alert was not acknowledged, do nothing.
		// Do nothing if state did not change.
		notify := func(notifications map[string]*conf.Notification) {
			for _, n := range notifications {
				s.Notify(state, n)
				checkNotify = true
			}
		}
		notifyCurrent := func() {
			state.NeedAck = true
			if _, present := silenced[ak]; present {
				return
			}
			switch event.Status {
			case StCritical, StUnknown:
				notify(a.CritNotification)
			case StWarning:
				notify(a.WarnNotification)
			}
		}
		clearOld := func() {
			state.NeedAck = false
			s.Lock()
			delete(s.Notifications, ak)
			s.Unlock()
		}
		if event.Status > last {
			clearOld()
			notifyCurrent()
		} else if event.Status < last {
			if _, hasOld := s.Notifications[ak]; hasOld {
				notifyCurrent()
			}
		}
	}
	return checkNotify
}

func (s *Schedule) CheckUnknown() {
	s.Lock()
	for ak, st := range s.status {
		if st.Forgotten {
			continue
		}
		t := s.Conf.Alerts[ak.Name()].Unknown
		if t == 0 {
			t = s.Conf.Unknown
		}
		if t == 0 {
			continue
		}
		if time.Since(st.Touched) < t {
			continue
		}
		s.RunHistory[ak] = Event{Status: StUnknown}
	}
	s.Unlock()
}

func (s *Schedule) CheckAlert(a *conf.Alert) {
	log.Printf("checking alert %v", a.Name)
	start := time.Now()
	var crits AlertKeys
	warns, err := s.CheckExpr(a, a.Warn, StWarning, nil)
	if err == nil {
		crits, _ = s.CheckExpr(a, a.Crit, StCritical, nil)
	}
	log.Printf("done checking alert %v (%s): %v crits, %v warns", a.Name, time.Since(start), len(crits), len(warns))
}

func (s *Schedule) CheckExpr(a *conf.Alert, e *expr.Expr, checkStatus Status, ignore AlertKeys) (alerts AlertKeys, err error) {
	if e == nil {
		return
	}
	defer func() {
		if err == nil {
			return
		}
		collect.Add("check.errs", opentsdb.TagSet{"metric": a.Name}, 1)
		log.Println(err)
	}()
	results, _, err := e.ExecuteOpts(s.cache, nil, s.CheckStart, 0)
	if err != nil {
		return
	}
Loop:
	for _, r := range results {
		if a.Squelched(r.Group) {
			continue
		}
		ak := NewAlertKey(a.Name, r.Group)
		for _, v := range ignore {
			if ak == v {
				continue Loop
			}
		}
		state := s.Status(ak)
		if state == nil {
			state = &State{
				Alert: ak.Name(),
				Tags:  r.Group.Tags(),
				Group: r.Group,
			}
			s.Lock()
			s.status[ak] = state
			s.Unlock()
		}
		state.Touch()
		status := checkStatus
		state.Computations = r.Computations
		var n float64
		switch v := r.Value.(type) {
		case expr.Number:
			n = float64(v)
		case expr.Scalar:
			n = float64(v)
		default:
			err = fmt.Errorf("expected number or scalar")
			return
		}
		var event Event
		if v, present := s.RunHistory[ak]; present {
			event = v
		} else {
			event = Event{Status: StNormal}
		}
		switch checkStatus {
		case StWarning:
			event.WarnResult = r
			event.WarnExpr = e.String()
		case StCritical:
			event.CritResult = r
			event.CritExpr = e.String()
		}
		event.Time = time.Now().UTC()
		if n != 0 {
			alerts = append(alerts, ak)
		}
		if n != 0 && status > s.RunHistory[ak].Status {
			event.Status = status
			s.RunHistory[ak] = event
		} else {
			s.RunHistory[ak] = event
		}
	}
	return
}
