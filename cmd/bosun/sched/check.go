package sched

import (
	"fmt"
	"log"
	"math"
	"time"

	"bosun.org/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"bosun.org/cmd/bosun/cache"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/expr"
	"bosun.org/collect"
	"bosun.org/graphite"
	"bosun.org/opentsdb"
)

func NewStatus(ak expr.AlertKey) *State {
	g := ak.Group()
	return &State{
		Alert: ak.Name(),
		Tags:  g.Tags(),
		Group: g,
	}
}

func (s *Schedule) Status(ak expr.AlertKey) *State {
	s.Lock()
	state := s.status[ak]
	if state == nil {
		state = NewStatus(ak)
		s.status[ak] = state
	}
	s.Unlock()
	return state
}

type RunHistory struct {
	Cache           *cache.Cache
	Start           time.Time
	Context         opentsdb.Context
	GraphiteContext graphite.Context
	Events          map[expr.AlertKey]*Event
}

// AtTime creates a new RunHistory starting at t with the same context and
// events as rh.
func (rh *RunHistory) AtTime(t time.Time) *RunHistory {
	n := *rh
	n.Start = t
	return &n
}

func (s *Schedule) NewRunHistory(start time.Time, cache *cache.Cache) *RunHistory {
	return &RunHistory{
		Cache:           cache,
		Start:           start,
		Events:          make(map[expr.AlertKey]*Event),
		Context:         s.Conf.TSDBContext(),
		GraphiteContext: s.Conf.GraphiteContext(),
	}
}

// Check evaluates all critical and warning alert rules. An error is returned if
// the check could not be performed.
func (s *Schedule) Check(T miniprofiler.Timer, now time.Time) (time.Duration, error) {
	select {
	case s.checkRunning <- true:
		// Good, we've got the lock.
	default:
		return 0, fmt.Errorf("check already running")
	}
	r := s.NewRunHistory(now, cache.New(0))
	start := time.Now()
	for _, a := range s.Conf.Alerts {
		s.CheckAlert(T, r, a)
	}
	d := time.Since(start)
	s.RunHistory(r)
	<-s.checkRunning
	return d, nil
}

// RunHistory processes an event history and trisggers notifications if needed.
func (s *Schedule) RunHistory(r *RunHistory) {
	checkNotify := false
	silenced := s.Silenced()
	s.Lock()
	defer s.Unlock()
	for ak, event := range r.Events {
		state := s.status[ak]
		if state == nil {
			state = NewStatus(ak)
			s.status[ak] = state
		}
		last := state.Append(event)
		a := s.Conf.Alerts[ak.Name()]
		if event.Status > StNormal {
			if event.Status != StUnknown {
				subject, serr := s.ExecuteSubject(r, a, state)
				if serr != nil {
					log.Printf("%s: %v", state.AlertKey(), serr)
				}
				body, _, berr := s.ExecuteBody(r, a, state, false)
				if berr != nil {
					log.Printf("%s: %v", state.AlertKey(), berr)
				}
				emailbody, attachments, merr := s.ExecuteBody(r, a, state, true)
				if merr != nil {
					log.Printf("%s: %v", state.AlertKey(), merr)
				}
				if serr != nil || berr != nil || merr != nil {
					var err error
					subject, body, err = s.ExecuteBadTemplate(serr, berr, r, a, state)
					if err != nil {
						subject = []byte(fmt.Sprintf("unable to create template error notification: %v", err))
					}
					emailbody = body
					attachments = nil
				}
				state.Subject = string(subject)
				state.Body = string(body)
				state.EmailBody = emailbody
				state.Attachments = attachments
			}
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
			// Auto close ignoreUnknowns.
			if a.IgnoreUnknown && event.Status == StUnknown {
				state.Open = false
				state.Forgotten = true
				state.NeedAck = false
				state.Action("bosun", "Auto close because alert has ignoreUnknown.", ActionClose)
				log.Printf("auto close %s because alert has ignoreUnknown", ak)
				return
			} else if silenced[ak].Forget && event.Status == StUnknown {
				state.Open = false
				state.Forgotten = true
				state.NeedAck = false
				state.Action("bosun", "Auto close because alert is silenced and marked auto forget.", ActionClose)
				log.Printf("auto close %s because alert is silenced and marked auto forget", ak)
				return
			}
			state.NeedAck = true
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
			// Auto close silenced alerts.
			if _, ok := silenced[ak]; ok && event.Status == StNormal {
				go func(ak expr.AlertKey) {
					log.Printf("auto close %s because was silenced", ak)
					err := s.Action("bosun", "Auto close because was silenced.", ActionClose, ak)
					if err != nil {
						log.Println(err)
					}
				}(ak)
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
	for range time.Tick(s.Conf.CheckFrequency / 4) {
		log.Println("checkUnknown")
		r := s.NewRunHistory(time.Now(), nil)
		s.Lock()
		for ak, st := range s.status {
			if st.Forgotten {
				continue
			}
			a := s.Conf.Alerts[ak.Name()]
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
			r.Events[ak] = &Event{Status: StUnknown}
		}
		s.Unlock()
		s.RunHistory(r)
	}
}

func (s *Schedule) CheckAlert(T miniprofiler.Timer, r *RunHistory, a *conf.Alert) {
	log.Printf("check alert %v start", a.Name)
	start := time.Now()
	var warns expr.AlertKeys
	crits, err := s.CheckExpr(T, r, a, a.Crit, StCritical, nil)
	if err == nil {
		warns, _ = s.CheckExpr(T, r, a, a.Warn, StWarning, crits)
	}
	collect.Put("check.duration", opentsdb.TagSet{"name": a.Name}, time.Since(start).Seconds())
	log.Printf("check alert %v done (%s): %v crits, %v warns", a.Name, time.Since(start), len(crits), len(warns))
}

func (s *Schedule) CheckExpr(T miniprofiler.Timer, rh *RunHistory, a *conf.Alert, e *expr.Expr, checkStatus Status, ignore expr.AlertKeys) (alerts expr.AlertKeys, err error) {
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
	results, _, err := e.Execute(rh.Context, rh.GraphiteContext, s.Conf.LogstashElasticHost, rh.Cache, T, rh.Start, 0, a.UnjoinedOK, s.Search, s.Conf.AlertSquelched(a))
	if err != nil {
		ak := expr.NewAlertKey(a.Name, nil)
		state := s.Status(ak)
		state.Touch()
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
		rh.Events[ak] = &Event{
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
		state.Touch()
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
		event := rh.Events[ak]
		if event == nil {
			event = new(Event)
			rh.Events[ak] = event
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
		if status > rh.Events[ak].Status {
			event.Status = status
			state.Result = &result
		}
	}
	return
}
