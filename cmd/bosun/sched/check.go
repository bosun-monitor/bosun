package sched

import (
	"fmt"
	"math"
	"time"

	"bosun.org/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"bosun.org/cmd/bosun/cache"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/expr"
	"bosun.org/collect"
	"bosun.org/graphite"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

func init() {
	metadata.AddMetricMeta(
		"bosun.alerts.current_severity", metadata.Gauge, metadata.Alert,
		"The number of open alerts by current severity.")
	metadata.AddMetricMeta(
		"bosun.alerts.last_abnormal_severity", metadata.Gauge, metadata.Alert,
		"The number of open alerts by last abnormal severity.")
	metadata.AddMetricMeta(
		"bosun.alerts.acknowledgement_status", metadata.Gauge, metadata.Alert,
		"The number of open alerts by acknowledgement status.")
	metadata.AddMetricMeta(
		"bosun.alerts.active_status", metadata.Gauge, metadata.Alert,
		"The number of open alerts by active status.")
	collect.AggregateMeta("bosun.template.render", metadata.MilliSecond, "The amount of time it takes to render the specified alert template.")
}

func NewStatus(ak expr.AlertKey) *State {
	g := ak.Group()
	return &State{
		Alert: ak.Name(),
		Tags:  g.Tags(),
		Group: g,
	}
}

// Get a copy of the status for the specified alert key
func (s *Schedule) GetStatus(ak expr.AlertKey) *State {
	s.Lock("GetStatus")
	state := s.status[ak]
	if state != nil {
		state = state.Copy()
	}
	s.Unlock()
	return state
}

func (s *Schedule) SetStatus(ak expr.AlertKey, st *State) {
	s.Lock("SetStatus")
	s.status[ak] = st
	s.Unlock()
}

func (s *Schedule) GetOrCreateStatus(ak expr.AlertKey) *State {
	s.Lock("GetOrCreateStatus")
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
	InfluxHost      string
	Logstash        expr.LogstashElasticHosts
	Events          map[expr.AlertKey]*Event
	schedule        *Schedule
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
		InfluxHost:      s.Conf.InfluxHost,
		Logstash:        s.Conf.LogstashElasticHosts,
		schedule:        s,
	}
}

// RunHistory processes an event history and triggers notifications if needed.
func (s *Schedule) RunHistory(r *RunHistory) {
	checkNotify := false
	silenced := s.Silenced()
	for ak, event := range r.Events {
		checkNotify = s.runHistory(r, ak, event, silenced) || checkNotify
	}
	if checkNotify && s.nc != nil {
		select {
		case s.nc <- true:
		default:
		}
	}
}

// RunHistory for a single alert key. Returns true if notifications were altered.
func (s *Schedule) runHistory(r *RunHistory, ak expr.AlertKey, event *Event, silenced map[expr.AlertKey]Silence) bool {
	checkNotify := false
	// get existing state object for alert key. add to schedule status if doesn't already exist
	state := s.GetStatus(ak)
	if state == nil {
		state = NewStatus(ak)
		s.SetStatus(ak, state)
	}
	defer s.SetStatus(ak, state)
	// make sure we always touch the state.
	state.Touched = r.Start
	// set state.Result according to event result
	if event.Error != nil {
		state.Result = event.Error
	} else if event.Crit != nil {
		state.Result = event.Crit
	} else if event.Warn != nil {
		state.Result = event.Warn
	}
	// if event is unevaluated, we are done.
	state.Unevaluated = event.Unevaluated
	if event.Unevaluated {
		return checkNotify
	}
	// assign incident id to new event if applicable
	prev := state.Last()
	event.Time = r.Start
	if prev.IncidentId != 0 {
		// If last event has incident id and is not closed, we continue it.
		s.incidentLock.Lock()
		if incident, ok := s.Incidents[prev.IncidentId]; ok && incident.End == nil {
			event.IncidentId = prev.IncidentId
		}
		s.incidentLock.Unlock()
	}
	if event.IncidentId == 0 && event.Status != StNormal {
		// Otherwise, create new incident on first non-normal event.
		event.IncidentId = s.createIncident(ak, event.Time).Id
	}
	// add new event to state
	last := state.AbnormalStatus()
	state.Append(event)
	a := s.Conf.Alerts[ak.Name()]
	wasOpen := state.Open
	// render templates and open alert key if abnormal
	if event.Status > StNormal {
		s.executeTemplates(state, event, a, r)
		state.Open = true
		if a.Log {
			state.Open = false
		}
	}
	// On state increase, clear old notifications and notify current.
	// On state decrease, and if the old alert was already acknowledged, notify current.
	// If the old alert was not acknowledged, do nothing.
	// Do nothing if state did not change.
	notify := func(ns *conf.Notifications) {
		if a.Log {
			lastLogTime := state.LastLogTime
			now := time.Now()
			if now.Before(lastLogTime.Add(a.MaxLogFrequency)) {
				return
			}
			state.LastLogTime = now
		}
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
			state.Action("bosun", "Auto close because alert has ignoreUnknown.", ActionClose, event.Time)
			slog.Infof("auto close %s because alert has ignoreUnknown", ak)
			return
		} else if silenced[ak].Forget && event.Status == StUnknown {
			state.Open = false
			state.Forgotten = true
			state.NeedAck = false
			state.Action("bosun", "Auto close because alert is silenced and marked auto forget.", ActionClose, event.Time)
			slog.Infof("auto close %s because alert is silenced and marked auto forget", ak)
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
	// lock while we change notifications.
	s.Lock("RunHistory")
	// last could be StNone if it is new. Set it to normal if so because StNormal >
	// StNone. If the state is not open (closed), then the last state we care about
	// isn't the last abnormal state, it's just normal.
	if last < StNormal || !wasOpen {
		last = StNormal
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
				slog.Infof("auto close %s because was silenced", ak)
				err := s.Action("bosun", "Auto close because was silenced.", ActionClose, ak)
				if err != nil {
					slog.Errorln(err)
				}
			}(ak)
		}
	}
	s.Unlock()
	return checkNotify
}

func (s *Schedule) executeTemplates(state *State, event *Event, a *conf.Alert, r *RunHistory) {
	state.Subject = ""
	state.Body = ""
	state.EmailBody = nil
	state.EmailSubject = nil
	state.Attachments = nil
	if event.Status != StUnknown {
		metric := "template.render"
		//Render subject
		endTiming := collect.StartTimer(metric, opentsdb.TagSet{"alert": a.Name, "type": "subject"})
		subject, serr := s.ExecuteSubject(r, a, state, false)
		if serr != nil {
			slog.Infof("%s: %v", state.AlertKey(), serr)
		}
		endTiming()
		//Render body
		endTiming = collect.StartTimer(metric, opentsdb.TagSet{"alert": a.Name, "type": "body"})
		body, _, berr := s.ExecuteBody(r, a, state, false)
		if berr != nil {
			slog.Infof("%s: %v", state.AlertKey(), berr)
		}
		endTiming()
		//Render email body
		endTiming = collect.StartTimer(metric, opentsdb.TagSet{"alert": a.Name, "type": "emailbody"})
		emailbody, attachments, merr := s.ExecuteBody(r, a, state, true)
		if merr != nil {
			slog.Infof("%s: %v", state.AlertKey(), merr)
		}
		endTiming()
		//Render email subject
		endTiming = collect.StartTimer(metric, opentsdb.TagSet{"alert": a.Name, "type": "emailsubject"})
		emailsubject, eserr := s.ExecuteSubject(r, a, state, true)
		endTiming()
		if serr != nil || berr != nil || merr != nil || eserr != nil {
			var err error

			endTiming = collect.StartTimer(metric, opentsdb.TagSet{"alert": a.Name, "type": "bad"})
			subject, body, err = s.ExecuteBadTemplate(serr, berr, r, a, state)
			endTiming()

			if err != nil {
				subject = []byte(fmt.Sprintf("unable to create template error notification: %v", err))
			}
			emailbody = body
			attachments = nil
		}
		state.Subject = string(subject)
		state.Body = string(body)
		state.EmailBody = emailbody
		state.EmailSubject = emailsubject
		state.Attachments = attachments
	}
}

// CollectStates sends various state information to bosun with collect.
func (s *Schedule) CollectStates() {
	// [AlertName][Severity]Count
	severityCounts := make(map[string]map[string]int64)
	abnormalCounts := make(map[string]map[string]int64)
	ackStatusCounts := make(map[string]map[bool]int64)
	activeStatusCounts := make(map[string]map[bool]int64)
	// Initalize the Counts
	for _, alert := range s.Conf.Alerts {
		severityCounts[alert.Name] = make(map[string]int64)
		abnormalCounts[alert.Name] = make(map[string]int64)
		var i Status
		for i = 1; i.String() != "none"; i++ {
			severityCounts[alert.Name][i.String()] = 0
			abnormalCounts[alert.Name][i.String()] = 0
		}
		ackStatusCounts[alert.Name] = make(map[bool]int64)
		activeStatusCounts[alert.Name] = make(map[bool]int64)
		ackStatusCounts[alert.Name][false] = 0
		activeStatusCounts[alert.Name][false] = 0
		ackStatusCounts[alert.Name][true] = 0
		activeStatusCounts[alert.Name][true] = 0
	}
	for _, state := range s.status {
		if !state.Open {
			continue
		}
		severity := state.Status().String()
		lastAbnormal := state.AbnormalStatus().String()
		severityCounts[state.Alert][severity]++
		abnormalCounts[state.Alert][lastAbnormal]++
		ackStatusCounts[state.Alert][state.NeedAck]++
		activeStatusCounts[state.Alert][state.IsActive()]++
	}
	for alertName := range severityCounts {
		ts := opentsdb.TagSet{"alert": alertName}
		// The tagset of the alert is not included because there is no way to
		// store the string of a group in OpenTSBD in a parsable way. This is
		// because any delimiter we chose could also be part of a tag key or tag
		// value.
		for severity := range severityCounts[alertName] {
			err := collect.Put("alerts.current_severity",
				ts.Copy().Merge(opentsdb.TagSet{"severity": severity}),
				severityCounts[alertName][severity])
			if err != nil {
				slog.Errorln(err)
			}
			err = collect.Put("alerts.last_abnormal_severity",
				ts.Copy().Merge(opentsdb.TagSet{"severity": severity}),
				abnormalCounts[alertName][severity])
			if err != nil {
				slog.Errorln(err)
			}
		}
		err := collect.Put("alerts.acknowledgement_status",
			ts.Copy().Merge(opentsdb.TagSet{"status": "unacknowledged"}),
			ackStatusCounts[alertName][true])
		err = collect.Put("alerts.acknowledgement_status",
			ts.Copy().Merge(opentsdb.TagSet{"status": "acknowledged"}),
			ackStatusCounts[alertName][false])
		if err != nil {
			slog.Errorln(err)
		}
		err = collect.Put("alerts.active_status",
			ts.Copy().Merge(opentsdb.TagSet{"status": "active"}),
			activeStatusCounts[alertName][true])
		if err != nil {
			slog.Errorln(err)
		}
		err = collect.Put("alerts.active_status",
			ts.Copy().Merge(opentsdb.TagSet{"status": "inactive"}),
			activeStatusCounts[alertName][false])
		if err != nil {
			slog.Errorln(err)
		}
	}
}

func (r *RunHistory) GetUnknownAndUnevaluatedAlertKeys(alert string) (unknown, uneval []expr.AlertKey) {
	unknown = []expr.AlertKey{}
	uneval = []expr.AlertKey{}
	r.schedule.Lock("GetUnknownUneval")
	for ak, st := range r.schedule.status {
		if ak.Name() != alert {
			continue
		}
		if st.Last().Status == StUnknown {
			unknown = append(unknown, ak)
		} else if st.Unevaluated {
			uneval = append(uneval, ak)
		}
	}
	r.schedule.Unlock()
	return unknown, uneval
}

var bosunStartupTime = time.Now()

func (s *Schedule) findUnknownAlerts(now time.Time, alert string) []expr.AlertKey {
	keys := []expr.AlertKey{}
	if time.Now().Sub(bosunStartupTime) < s.Conf.CheckFrequency {
		return keys
	}
	s.Lock("FindUnknown")
	for ak, st := range s.status {
		name := ak.Name()
		if name != alert || st.Forgotten || st.Status() == StError {
			continue
		}
		a := s.Conf.Alerts[name]
		t := a.Unknown
		if t == 0 {
			t = s.Conf.CheckFrequency * 2 * time.Duration(a.RunEvery)
		}
		if now.Sub(st.Touched) < t {
			continue
		}
		keys = append(keys, ak)
	}
	s.Unlock()
	return keys
}

func (s *Schedule) CheckAlert(T miniprofiler.Timer, r *RunHistory, a *conf.Alert) {
	slog.Infof("check alert %v start", a.Name)
	start := time.Now()
	for _, ak := range s.findUnknownAlerts(r.Start, a.Name) {
		r.Events[ak] = &Event{Status: StUnknown}
	}
	var warns, crits expr.AlertKeys
	d, err := s.executeExpr(T, r, a, a.Depends)
	var deps expr.ResultSlice
	if err == nil {
		deps = filterDependencyResults(d)
		crits, err = s.CheckExpr(T, r, a, a.Crit, StCritical, nil)
		if err == nil {
			warns, _ = s.CheckExpr(T, r, a, a.Warn, StWarning, crits)
		}
	}
	unevalCount, unknownCount := markDependenciesUnevaluated(r.Events, deps, a.Name)
	if err != nil {
		slog.Errorf("Error checking alert %s: %s", a.Name, err.Error())
		removeUnknownEvents(r.Events, a.Name)
	}
	collect.Put("check.duration", opentsdb.TagSet{"name": a.Name}, time.Since(start).Seconds())
	slog.Infof("check alert %v done (%s): %v crits, %v warns, %v unevaluated, %v unknown", a.Name, time.Since(start), len(crits), len(warns), unevalCount, unknownCount)
}

func removeUnknownEvents(evs map[expr.AlertKey]*Event, alert string) {
	for k, v := range evs {
		if v.Status == StUnknown && k.Name() == alert {
			delete(evs, k)
		}
	}
}

func filterDependencyResults(results *expr.Results) expr.ResultSlice {
	// take the results of the dependency expression and filter it to
	// non-zero tag sets.
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

func markDependenciesUnevaluated(events map[expr.AlertKey]*Event, deps expr.ResultSlice, alert string) (unevalCount, unknownCount int) {
	for ak, ev := range events {
		if ak.Name() != alert {
			continue
		}
		for _, dep := range deps {
			if dep.Group.Overlaps(ak.Group()) {
				ev.Unevaluated = true
				unevalCount++
			}
			if ev.Status == StUnknown {
				unknownCount++
			}
		}
	}
	return unevalCount, unknownCount
}

func (s *Schedule) executeExpr(T miniprofiler.Timer, rh *RunHistory, a *conf.Alert, e *expr.Expr) (*expr.Results, error) {
	if e == nil {
		return nil, nil
	}
	results, _, err := e.Execute(rh.Context, rh.GraphiteContext, rh.Logstash, rh.InfluxHost, rh.Cache, T, rh.Start, 0, a.UnjoinedOK, s.Search, s.Conf.AlertSquelched(a), rh)
	if err != nil {
		ak := expr.NewAlertKey(a.Name, nil)
		rh.Events[ak] = &Event{
			Status: StError,
			Error: &Result{
				Result: &expr.Result{
					Computations: []expr.Computation{
						{
							Text:  e.String(),
							Value: err.Error(),
						},
					},
				},
			},
		}
		return nil, err
	}
	return results, err
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
		slog.Errorln(err)
	}()
	results, err := s.executeExpr(T, rh, a, e)
	if err != nil {
		return nil, err
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
		result := &Result{
			Result: r,
			Expr:   e.String(),
		}
		switch checkStatus {
		case StWarning:
			event.Warn = result
		case StCritical:
			event.Crit = result
		}
		status := checkStatus
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
		}
	}
	return
}
