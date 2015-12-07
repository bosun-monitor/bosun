package sched

import (
	"fmt"
	"math"
	"time"

	"bosun.org/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"bosun.org/_third_party/github.com/influxdb/influxdb/client"
	"bosun.org/cmd/bosun/cache"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/expr"
	"bosun.org/collect"
	"bosun.org/graphite"
	"bosun.org/metadata"
	"bosun.org/models"
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
	metadata.AddMetricMeta("alerts.acknowledgement_status_by_notification", metadata.Gauge, metadata.Alert,
		"The number of alerts by acknowledgement status and notification. Does not reflect escalation chains.")
	metadata.AddMetricMeta("alerts.oldest_unacked_by_notification", metadata.Gauge, metadata.Second,
		"How old the oldest unacknowledged notification is by notification.. Does not reflect escalation chains.")
	collect.AggregateMeta("bosun.template.render", metadata.MilliSecond, "The amount of time it takes to render the specified alert template.")
}

func NewStatus(ak models.AlertKey) *State {
	g := ak.Group()
	return &State{
		Alert: ak.Name(),
		Tags:  g.Tags(),
		Group: g,
	}
}

// Get a copy of the status for the specified alert key
func (s *Schedule) GetStatus(ak models.AlertKey) *State {
	s.Lock("GetStatus")
	state := s.status[ak]
	if state != nil {
		state = state.Copy()
	}
	s.Unlock()
	return state
}

func (s *Schedule) SetStatus(ak models.AlertKey, st *State) {
	s.Lock("SetStatus")
	s.status[ak] = st
	s.Unlock()
}

func (s *Schedule) GetOrCreateStatus(ak models.AlertKey) *State {
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
	InfluxConfig    client.Config
	Logstash        expr.LogstashElasticHosts
	Events          map[models.AlertKey]*Event
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
		Events:          make(map[models.AlertKey]*Event),
		Context:         s.Conf.TSDBContext(),
		GraphiteContext: s.Conf.GraphiteContext(),
		InfluxConfig:    s.Conf.InfluxConfig,
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
func (s *Schedule) runHistory(r *RunHistory, ak models.AlertKey, event *Event, silenced map[models.AlertKey]models.Silence) bool {
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
	if event.Crit != nil {
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
	worst := StNormal
	event.Time = r.Start
	if prev.IncidentId != 0 {
		// If last event has incident id and is not closed, we continue it.
		incident, err := s.DataAccess.Incidents().GetIncident(prev.IncidentId)
		if err != nil {
			slog.Error(err)
		} else if incident.End == nil {
			event.IncidentId = prev.IncidentId
			worst = state.WorstThisIncident()
		}
	}
	if event.IncidentId == 0 && event.Status != StNormal {
		incident, err := s.createIncident(ak, event.Time)
		if err != nil {
			slog.Error("Error creating incident", err)
		} else {
			event.IncidentId = incident.Id
		}
	}

	state.Append(event)
	a := s.Conf.Alerts[ak.Name()]
	// render templates and open alert key if abnormal
	if event.Status > StNormal {
		s.executeTemplates(state, event, a, r)
		state.Open = true
		if a.Log {
			worst = StNormal
			state.Open = false
		}
	}
	// On state increase, clear old notifications and notify current.
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
	if event.Status > worst {
		clearOld()
		notifyCurrent()
	} else if _, ok := silenced[ak]; ok && event.Status == StNormal {
		go func(ak models.AlertKey) {
			slog.Infof("auto close %s because was silenced", ak)
			err := s.Action("bosun", "Auto close because was silenced.", ActionClose, ak)
			if err != nil {
				slog.Errorln(err)
			}
		}(ak)
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
		var errs []error
		metric := "template.render"
		//Render subject
		endTiming := collect.StartTimer(metric, opentsdb.TagSet{"alert": a.Name, "type": "subject"})
		subject, err := s.ExecuteSubject(r, a, state, false)
		if err != nil {
			slog.Infof("%s: %v", state.AlertKey(), err)
			errs = append(errs, err)
		} else if subject == nil {
			err = fmt.Errorf("Empty subject on %s", state.AlertKey())
			slog.Error(err)
			errs = append(errs, err)
		}
		endTiming()

		//Render body
		endTiming = collect.StartTimer(metric, opentsdb.TagSet{"alert": a.Name, "type": "body"})
		body, _, err := s.ExecuteBody(r, a, state, false)
		if err != nil {
			slog.Infof("%s: %v", state.AlertKey(), err)
			errs = append(errs, err)
		} else if subject == nil {
			err = fmt.Errorf("Empty body on %s", state.AlertKey())
			slog.Error(err)
			errs = append(errs, err)
		}
		endTiming()

		//Render email body
		endTiming = collect.StartTimer(metric, opentsdb.TagSet{"alert": a.Name, "type": "emailbody"})
		emailbody, attachments, err := s.ExecuteBody(r, a, state, true)
		if err != nil {
			slog.Infof("%s: %v", state.AlertKey(), err)
			errs = append(errs, err)
		} else if subject == nil {
			err = fmt.Errorf("Empty email body on %s", state.AlertKey())
			slog.Error(err)
			errs = append(errs, err)
		}
		endTiming()

		//Render email subject
		endTiming = collect.StartTimer(metric, opentsdb.TagSet{"alert": a.Name, "type": "emailsubject"})
		emailsubject, err := s.ExecuteSubject(r, a, state, true)
		if err != nil {
			slog.Infof("%s: %v", state.AlertKey(), err)
			errs = append(errs, err)
		} else if subject == nil {
			err = fmt.Errorf("Empty email subject on %s", state.AlertKey())
			slog.Error(err)
			errs = append(errs, err)
		}
		endTiming()

		if errs != nil {
			endTiming = collect.StartTimer(metric, opentsdb.TagSet{"alert": a.Name, "type": "bad"})
			subject, body, err = s.ExecuteBadTemplate(errs, r, a, state)
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
	ackByNotificationCounts := make(map[string]map[bool]int64)
	unAckOldestByNotification := make(map[string]time.Time)
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
	for notificationName := range s.Conf.Notifications {
		unAckOldestByNotification[notificationName] = time.Unix(1<<63-62135596801, 999999999)
		ackByNotificationCounts[notificationName] = make(map[bool]int64)
		ackByNotificationCounts[notificationName][false] = 0
		ackByNotificationCounts[notificationName][true] = 0
	}
	for _, state := range s.status {
		if !state.Open {
			continue
		}
		name := state.AlertKey().Name()
		alertDef := s.Conf.Alerts[name]
		nots := make(map[string]bool)
		for name := range alertDef.WarnNotification.Get(s.Conf, state.Group) {
			nots[name] = true
		}
		for name := range alertDef.CritNotification.Get(s.Conf, state.Group) {
			nots[name] = true
		}
		incident, err := s.GetIncident(state.Last().IncidentId)
		if err != nil {
			slog.Errorln(err)
		}
		for notificationName := range nots {
			ackByNotificationCounts[notificationName][state.NeedAck]++
			if incident != nil && incident.Start.Before(unAckOldestByNotification[notificationName]) && state.NeedAck {
				unAckOldestByNotification[notificationName] = incident.Start
			}
		}
		severity := state.Status().String()
		lastAbnormal := state.AbnormalStatus().String()
		severityCounts[state.Alert][severity]++
		abnormalCounts[state.Alert][lastAbnormal]++
		ackStatusCounts[state.Alert][state.NeedAck]++
		activeStatusCounts[state.Alert][state.IsActive()]++
	}
	for notification := range ackByNotificationCounts {
		ts := opentsdb.TagSet{"notification": notification}
		err := collect.Put("alerts.acknowledgement_status_by_notification",
			ts.Copy().Merge(opentsdb.TagSet{"status": "unacknowledged"}),
			ackByNotificationCounts[notification][true])
		if err != nil {
			slog.Errorln(err)
		}
		err = collect.Put("alerts.acknowledgement_status_by_notification",
			ts.Copy().Merge(opentsdb.TagSet{"status": "acknowledged"}),
			ackByNotificationCounts[notification][false])
		if err != nil {
			slog.Errorln(err)
		}
	}
	for notification, timeStamp := range unAckOldestByNotification {
		ts := opentsdb.TagSet{"notification": notification}
		var ago time.Duration
		if !timeStamp.Equal(time.Unix(1<<63-62135596801, 999999999)) {
			ago = time.Now().UTC().Sub(timeStamp)
		}
		err := collect.Put("alerts.oldest_unacked_by_notification",
			ts,
			ago.Seconds())
		if err != nil {
			slog.Errorln(err)
		}
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

func (r *RunHistory) GetUnknownAndUnevaluatedAlertKeys(alert string) (unknown, uneval []models.AlertKey) {
	unknown = []models.AlertKey{}
	uneval = []models.AlertKey{}
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

func (s *Schedule) findUnknownAlerts(now time.Time, alert string) []models.AlertKey {
	keys := []models.AlertKey{}
	if time.Now().Sub(bosunStartupTime) < s.Conf.CheckFrequency {
		return keys
	}
	s.Lock("FindUnknown")
	for ak, st := range s.status {
		name := ak.Name()
		if name != alert || st.Forgotten || !s.AlertSuccessful(ak.Name()) {
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
	var warns, crits models.AlertKeys
	d, err := s.executeExpr(T, r, a, a.Depends)
	var deps expr.ResultSlice
	if err == nil {
		deps = filterDependencyResults(d)
		crits, err = s.CheckExpr(T, r, a, a.Crit, StCritical, nil)
		if err == nil {
			warns, err = s.CheckExpr(T, r, a, a.Warn, StWarning, crits)
		}
	}
	unevalCount, unknownCount := markDependenciesUnevaluated(r.Events, deps, a.Name)
	if err != nil {
		slog.Errorf("Error checking alert %s: %s", a.Name, err.Error())
		removeUnknownEvents(r.Events, a.Name)
		s.markAlertError(a.Name, err)
	} else {
		s.markAlertSuccessful(a.Name)
	}
	collect.Put("check.duration", opentsdb.TagSet{"name": a.Name}, time.Since(start).Seconds())
	slog.Infof("check alert %v done (%s): %v crits, %v warns, %v unevaluated, %v unknown", a.Name, time.Since(start), len(crits), len(warns), unevalCount, unknownCount)
}

func removeUnknownEvents(evs map[models.AlertKey]*Event, alert string) {
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

func markDependenciesUnevaluated(events map[models.AlertKey]*Event, deps expr.ResultSlice, alert string) (unevalCount, unknownCount int) {
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
	results, _, err := e.Execute(rh.Context, rh.GraphiteContext, rh.Logstash, rh.InfluxConfig, rh.Cache, T, rh.Start, 0, a.UnjoinedOK, s.Search, s.Conf.AlertSquelched(a), rh)
	return results, err
}

func (s *Schedule) CheckExpr(T miniprofiler.Timer, rh *RunHistory, a *conf.Alert, e *expr.Expr, checkStatus Status, ignore models.AlertKeys) (alerts models.AlertKeys, err error) {
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
		ak := models.NewAlertKey(a.Name, r.Group)
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
			status = checkStatus
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
