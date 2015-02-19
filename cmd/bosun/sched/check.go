package sched

import (
	"fmt"
	"log"
	"math"
	"time"

	"bosun.org/_third_party/github.com/MiniProfiler/go/miniprofiler"
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
	Start           time.Time
	Context         opentsdb.Context
	GraphiteContext graphite.Context

	Events map[expr.AlertKey]*Event

	anyChanges bool // will be set to true in addEvent if anything has changed since the last time the flag was cleared
}

// AtTime creates a new RunHistory starting at t with the same context and
// events as rh.
func (rh *RunHistory) AtTime(t time.Time) *RunHistory {
	n := *rh
	n.Start = t
	return &n
}

// Returns all tagsets for the given alert name that have a status of Unkown or are Unevaluated.
// Implements expr.AlertStatusProvider
func (rh *RunHistory) GetUnknownTagSets(alert string) (unknown []opentsdb.TagSet, unevaluated []opentsdb.TagSet) {
	unknown = []opentsdb.TagSet{}
	uneval := []opentsdb.TagSet{}
	for k, v := range rh.Events {
		if k.Name() == alert {
			if v.Unevaluated {
				uneval = append(uneval, k.Group())
			} else if v.Status == StUnknown {
				unknown = append(unknown, k.Group())
			}
		}
	}
	return unknown, uneval
}

func (rh *RunHistory) addEvent(ak expr.AlertKey, e *Event) {
	current := rh.Events[ak]
	if current != nil {
		if current.Status != e.Status || current.Unevaluated != e.Unevaluated {
			rh.anyChanges = true
		}
	}
	rh.Events[ak] = e
}

func (s *Schedule) NewRunHistory(start time.Time) *RunHistory {
	return &RunHistory{
		Start:           start,
		Events:          make(map[expr.AlertKey]*Event),
		Context:         s.Conf.TSDBCacheContext(),
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
	r := s.NewRunHistory(now)
	r.Events = s.getUnknownAlertKeys()

	start := time.Now()
	r.anyChanges = true
	for r.anyChanges {
		r.anyChanges = false
		for {
			for _, a := range s.Conf.Alerts {
				s.CheckAlert(T, r, a)
			}
			break
		}
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
		if event.Unevaluated {
			state.Unevaluated = true
			continue
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

// Looks for alert keys that have not been touched recently.
func (s *Schedule) getUnknownAlertKeys() map[expr.AlertKey]*Event {
	events := map[expr.AlertKey]*Event{}
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
		if time.Since(st.Touched) < t {
			continue
		}
		events[ak] = &Event{Status: StUnknown}
	}
	s.Unlock()
	return events
}

func (s *Schedule) CheckAlert(T miniprofiler.Timer, r *RunHistory, a *conf.Alert) {
	log.Printf("check alert %v start", a.Name)
	start := time.Now()
	var warns, crits expr.AlertKeys
	ignore := map[expr.AlertKey]bool{}
	d, err := s.executeExpr(T, r, a, a.Depends)
	dependencyResults := filterDependencyResults(d)
	if err != nil {
		collect.Add("check.errs", opentsdb.TagSet{"metric": a.Name}, 1)
		log.Println(err)
	} else {
		crits, err = s.CheckExpr(T, r, a, a.Crit, StCritical, ignore, dependencyResults)
		for _, ak := range crits {
			ignore[ak] = true
		}
		if err == nil {
			warns, _ = s.CheckExpr(T, r, a, a.Warn, StWarning, ignore, dependencyResults)
		}
	}
	collect.Put("check.duration", opentsdb.TagSet{"name": a.Name}, time.Since(start).Seconds())
	log.Printf("check alert %v done (%s): %v crits, %v warns, %v ignored because of dependencies.", a.Name, time.Since(start), len(crits), len(warns), len(dependencyResults))
}

func filterDependencyResults(results *expr.Results) expr.ResultSlice {
	filtered := expr.ResultSlice{}
	if results == nil {
		return filtered
	}
	for _, r := range results.Results {
		var n float64
		switch v := r.Value.(type) {
		case expr.Number:
			n = float64(v)
		case expr.Scalar:
			n = float64(v)
		}
		if !math.IsNaN(n) && n != 0 {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func (s *Schedule) executeExpr(T miniprofiler.Timer, rh *RunHistory, a *conf.Alert, e *expr.Expr) (*expr.Results, error) {
	if e == nil {
		return nil, nil
	}
	results, _, err := e.Execute(rh.Context, rh.GraphiteContext, s.Conf.LogstashElasticHost, T, rh.Start, 0, a.UnjoinedOK, s.Search, s.Conf.AlertSquelched(a), rh)
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
		rh.addEvent(ak, &Event{
			Status: StError,
		})
		return nil, err
	}
	return results, err
}

func (s *Schedule) CheckExpr(T miniprofiler.Timer, rh *RunHistory, a *conf.Alert, e *expr.Expr, checkStatus Status, ignore map[expr.AlertKey]bool, dependencies expr.ResultSlice) (alerts expr.AlertKeys, err error) {
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
	results, err := s.executeExpr(T, rh, a, e)
	if err != nil {
		return nil, err
	}
	for _, r := range results.Results {
		if s.Conf.Squelched(a, r.Group) {
			continue
		}
		ak := expr.NewAlertKey(a.Name, r.Group)
		if ignore[ak] {
			continue
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
		current := rh.Events[ak]
		event := &Event{}
		unevaluated := false
		for _, dep := range dependencies {
			if ak.Group().Overlaps(dep.Group) {
				event.Unevaluated = true
				if current != nil {
					event.Status = current.Status
				} else {
					event.Status = StNormal
				}
				unevaluated = true
				break
			}
		}
		if !unevaluated {
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
		rh.addEvent(ak, event)
	}
	return
}
