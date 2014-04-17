package sched

import (
	"bytes"
	"log"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/expr"
)

func (s *Schedule) Check() {
	s.Lock()
	defer s.Unlock()
	s.runStates = make(map[AlertKey]Status)
	s.cache = opentsdb.NewCache(s.Conf.TsdbHost)
	for _, a := range s.Conf.Alerts {
		s.CheckAlert(a)
	}
	s.CheckUnknown()
	checkNotify := false
	silenced := s.Silenced()
	for ak, status := range s.runStates {
		state := s.Status[ak]
		last := state.Append(status)
		a := s.Conf.Alerts[ak.Name]
		if status > stNormal {
			var subject = new(bytes.Buffer)
			if err := s.ExecuteSubject(subject, a, state); err != nil {
				log.Println(err)
			}
			state.Subject = subject.String()
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
				log.Println("SILENCED", ak)
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
			delete(s.Notifications, ak)
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
	for ak, st := range s.Status {
		t := s.Conf.Alerts[ak.Name].Unknown
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
}

func (s *Schedule) CheckAlert(a *conf.Alert) {
	crits := s.CheckExpr(a, a.Crit, stCritical, nil)
	warns := s.CheckExpr(a, a.Warn, stWarning, crits)
	log.Printf("checking alert %v: %v crits, %v warns", a.Name, len(crits), len(warns))
}

func (s *Schedule) CheckExpr(a *conf.Alert, e *expr.Expr, checkStatus Status, ignore AlertKeys) (alerts AlertKeys) {
	if e == nil {
		return
	}
	results, _, err := e.Execute(s.cache, nil)
	if err != nil {
		// todo: do something here?
		log.Println(err)
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
		state := s.Status[ak]
		if state == nil {
			state = &State{
				Alert: ak.Name,
				Tags:  r.Group.Tags(),
				Group: r.Group,
			}
			s.Status[ak] = state
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
			panic("expected number or scalar")
		}
		if n != 0 {
			state.Expr = e.String()
			alerts = append(alerts, ak)
		} else {
			status = stNormal
		}
		if status > s.runStates[ak] {
			s.runStates[ak] = status
		}
	}
	return
}
