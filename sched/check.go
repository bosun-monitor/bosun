package sched

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/collect"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/bosun/conf"
	"github.com/StackExchange/bosun/expr"
)

func (s *Schedule) Status(ak expr.AlertKey) *State {
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

type RunHistory map[expr.AlertKey]*Event

// Check evaluates all critical and warning alert rules.
func (s *Schedule) Check(T miniprofiler.Timer, start time.Time) {
	r := make(RunHistory)
	s.CheckStart = start
	s.cache = opentsdb.NewCache(s.Conf.TsdbHost, s.Conf.ResponseLimit)
	for _, a := range s.Conf.Alerts {
		s.CheckAlert(T, r, a)
	}
	s.RunHistory(r)
}

// RunHistory processes an event history and trisggers notifications if needed.
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
		notify := func(ns *conf.Notifications) {
			nots := ns.Get(s.Conf, state.Group)
			for _, n := range nots {
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
	if checkNotify && s.nc != nil {
		s.nc <- true
	}
	s.Save()
}

// CheckUnknown checks for unknown alerts.
func (s *Schedule) CheckUnknown() {
	for _ = range time.Tick(s.Conf.CheckFrequency / 4) {
		log.Println("checkUnknown")
		r := make(RunHistory)
		s.Lock()
		for ak, st := range s.status {
			if st.Forgotten {
				continue
			}
			a := s.Conf.Alerts[ak.Name()]
			if a.IgnoreUnknown {
				continue
			}
			t := a.Unknown
			if t == 0 {
				t = s.Conf.CheckFrequency * 2
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

func (s *Schedule) CheckAlert(T miniprofiler.Timer, r RunHistory, a *conf.Alert) {
	log.Printf("checking alert %v", a.Name)
	start := time.Now()
	var warns expr.AlertKeys
	crits, err := s.CheckExpr(T, r, a, a.Crit, StCritical, nil)
	if err == nil {
		warns, _ = s.CheckExpr(T, r, a, a.Warn, StWarning, crits)
	}
	collect.Put("check.duration", opentsdb.TagSet{"name": a.Name}, time.Since(start).Seconds())
	log.Printf("done checking alert %v (%s): %v crits, %v warns", a.Name, time.Since(start), len(crits), len(warns))
}

func (s *Schedule) CheckExpr(T miniprofiler.Timer, rh RunHistory, a *conf.Alert, e *expr.Expr, checkStatus Status, ignore expr.AlertKeys) (alerts expr.AlertKeys, err error) {
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
	results, _, err := e.Execute(s.cache, T, s.CheckStart, 0, a.UnjoinedOK, s.Search, s.Conf.GetLookups(), s.Conf.AlertSquelched(a))
	if err != nil {
		ak := expr.NewAlertKey(a.Name, nil)
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
	for _, r := range results.Results {
		if s.Conf.Squelched(a, r.Group) {
			continue
		}
		ak := expr.NewAlertKey(a.Name, r.Group)
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
