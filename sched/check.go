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
	s.CheckStart = time.Now().UTC()
	s.runStates = make(map[AlertKey]Status)
	s.cache = opentsdb.NewCache(s.Conf.TsdbHost, s.Conf.ResponseLimit)
	for _, a := range s.Conf.Alerts {
		s.CheckAlert(a)
	}
	s.CheckUnknown()
	checkNotify := false
	silenced := s.Silenced()
	for ak, status := range s.runStates {
		state := s.Status(ak)
		last := state.Append(status)
		a := s.Conf.Alerts[ak.Name()]
		if status > stNormal {
			var subject = new(bytes.Buffer)
			if status != stUnknown {
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
			switch status {
			case stCritical, stUnknown:
				notify(a.CritNotification)
			case stWarning:
				notify(a.WarnNotification)
			}
		}
		clearOld := func() {
			state.NeedAck = false
			s.Lock()
			delete(s.Notifications, ak)
			s.Unlock()
		}
		if status > last {
			clearOld()
			notifyCurrent()
		} else if status < last {
			if _, hasOld := s.Notifications[ak]; hasOld {
				notifyCurrent()
			}
		}
	}
	if checkNotify {
		s.nc <- true
	}
	s.Save()
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
		s.runStates[ak] = stUnknown
	}
	s.Unlock()
}

func (s *Schedule) CheckAlert(a *conf.Alert) {
	log.Printf("checking alert %v", a.Name)
	start := time.Now()
	crits, _, err := s.CheckExpr(a, a.Crit, stCritical, nil)
	var warns AlertKeys
	if err == nil {
		warns, _, _ = s.CheckExpr(a, a.Warn, stWarning, crits)
	}
	log.Printf("done checking alert %v (%s): %v crits, %v warns", a.Name, time.Since(start), len(crits), len(warns))
}

func (s *Schedule) CheckExpr(a *conf.Alert, e *expr.Expr, checkStatus Status, ignore AlertKeys) (alerts AlertKeys, oks AlertKeys, err error) {
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
		if n != 0 {
			state.Expr = e.String()
			alerts = append(alerts, ak)
		} else {
			status = stNormal
			oks = append(oks, ak)
		}
		if s.runStates != nil && status > s.runStates[ak] {
			s.runStates[ak] = status
		}
	}
	return
}
