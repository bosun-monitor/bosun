package sched

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/collect"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/bosun/conf"
	"github.com/StackExchange/bosun/expr"
)

func (s *Schedule) Status(ak AlertKey) *State {
	s.Lock()
	state := s.status[ak]
	if state == nil {
		g := ak.Group()
		state = &State{
			Alert: ak.Name(),
			Tags:  g.Tags(),
			Group: g,
		}
		s.status[ak] = state
	}
	state.Touch()
	s.Unlock()
	return state
}

type RunHistory map[AlertKey]*Event

// Check evaluates all critical and warning alert rules.
func (s *Schedule) Check() {
	r := make(RunHistory)
	s.CheckStart = time.Now().UTC()
	s.cache = opentsdb.NewCache(s.Conf.TsdbHost, s.Conf.ResponseLimit)
	for _, a := range s.Conf.Alerts {
		s.CheckAlert(r, a)
	}
	s.RunHistory(r)
}

// RunHistory processes an event history and triggers notifications if needed.
func (s *Schedule) RunHistory(r RunHistory) {
	checkNotify := false
	silenced := s.Silenced()
	s.Lock()
	defer s.Unlock()
	for ak, event := range r {
		state := s.status[ak]
		last := state.Append(event)
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
			delete(s.Notifications, ak)
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
	if checkNotify {
		s.nc <- true
	}
	s.Save()
}

// CheckUnknown checks for unknown alerts.
func (s *Schedule) CheckUnknown() {
	for _ = range time.Tick(s.Conf.CheckFrequency) {
		log.Println("checkUnknown")
		r := make(RunHistory)
		s.Lock()
		for ak, st := range s.status {
			if st.Forgotten {
				continue
			}
			t := s.Conf.Alerts[ak.Name()].Unknown
			if t == 0 {
				t = s.Conf.CheckFrequency
			}
			if t == 0 {
				continue
			}
			if time.Since(st.Touched) < t {
				continue
			}
			r[ak] = &Event{Status: StUnknown}
		}
		s.Unlock()
		s.RunHistory(r)
	}
}

func (s *Schedule) CheckAlert(r RunHistory, a *conf.Alert) {
	log.Printf("checking alert %v", a.Name)
	start := time.Now()
	var warns AlertKeys
	crits, err := s.CheckExpr(r, a, a.Crit, StCritical, nil)
	if err == nil {
		warns, _ = s.CheckExpr(r, a, a.Warn, StWarning, crits)
	}
	log.Printf("done checking alert %v (%s): %v crits, %v warns", a.Name, time.Since(start), len(crits), len(warns))
}

func (s *Schedule) CheckExpr(rh RunHistory, a *conf.Alert, e *expr.Expr, checkStatus Status, ignore AlertKeys) (alerts AlertKeys, err error) {
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
		ak := NewAlertKey(a.Name, nil)
		state := s.Status(ak)
		state.Result = &Result{
			Result: &expr.Result{
				Computations: []expr.Computation{
					{
						Text:  e.String(),
						Value: err.Error(),
					},
				},
			},
		}
		rh[ak] = &Event{
			Status: StError,
		}
		return
	}
Loop:
	for _, r := range results {
		if s.Conf.Squelched(a, r.Group) {
			continue
		}
		ak := NewAlertKey(a.Name, r.Group)
		for _, v := range ignore {
			if ak == v {
				continue Loop
			}
		}
		state := s.Status(ak)
		status := checkStatus
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
		event := rh[ak]
		if event == nil {
			event = new(Event)
			rh[ak] = event
		}
		result := Result{
			Result: r,
			Expr:   e.String(),
		}
		switch checkStatus {
		case StWarning:
			event.Warn = &result
		case StCritical:
			event.Crit = &result
		}
		if math.IsNaN(n) {
			status = StError
		} else if n == 0 {
			status = StNormal
		}
		if status != StNormal {
			alerts = append(alerts, ak)
		}
		if status > rh[ak].Status {
			event.Status = status
			state.Result = &result
		}
	}
	return
}
