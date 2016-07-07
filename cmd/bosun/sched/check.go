package sched

import (
	"fmt"
	"math"
	"time"

	"bosun.org/cmd/bosun/cache"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/expr"
	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"github.com/MiniProfiler/go/miniprofiler"
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

func NewIncident(ak models.AlertKey) *models.IncidentState {
	s := &models.IncidentState{}
	s.Start = utcNow()
	s.AlertKey = ak
	s.Alert = ak.Name()
	s.Tags = ak.Group().Tags()
	s.Result = &models.Result{}
	return s
}

type RunHistory struct {
	Cache    *cache.Cache
	Start    time.Time
	Backends *expr.Backends
	Events   map[models.AlertKey]*models.Event
	schedule *Schedule
}

// AtTime creates a new RunHistory starting at t with the same context and
// events as rh.
func (rh *RunHistory) AtTime(t time.Time) *RunHistory {
	n := *rh
	n.Start = t
	return &n
}

func (s *Schedule) NewRunHistory(start time.Time, cache *cache.Cache) *RunHistory {
	r := &RunHistory{
		Cache:    cache,
		Start:    start,
		Events:   make(map[models.AlertKey]*models.Event),
		schedule: s,
		Backends: &expr.Backends{
			TSDBContext:     s.Conf.GetTSDBContext(),
			GraphiteContext: s.Conf.GetGraphiteContext(),
			InfluxConfig:    s.Conf.GetInfluxContext(),
			LogstashHosts:   s.Conf.GetLogstashContext(),
			ElasticHosts:    s.Conf.GetElasticContext(),
		},
	}
	return r
}

// RunHistory processes an event history and triggers notifications if needed.
func (s *Schedule) RunHistory(r *RunHistory) {
	checkNotify := false
	silenced := s.Silenced()
	for ak, event := range r.Events {
		shouldNotify, err := s.runHistory(r, ak, event, silenced)
		checkNotify = checkNotify || shouldNotify
		if err != nil {
			slog.Errorf("Error in runHistory for %s. %s.", ak, err)
		}
	}
	if checkNotify && s.nc != nil {
		select {
		case s.nc <- true:
		default:
		}
	}
}

// RunHistory for a single alert key. Returns true if notifications were altered.
func (s *Schedule) runHistory(r *RunHistory, ak models.AlertKey, event *models.Event, silenced SilenceTester) (checkNotify bool, err error) {
	event.Time = r.Start
	a := s.Conf.GetAlert(ak.Name())
	if a.UnknownsNormal && event.Status == models.StUnknown {
		event.Status = models.StNormal
	}

	data := s.DataAccess.State()
	err = data.TouchAlertKey(ak, utcNow())
	if err != nil {
		return
	}

	si := silenced(ak)

	// get existing open incident if exists
	var incident *models.IncidentState
	incident, err = data.GetOpenIncident(ak)
	if err != nil {
		return
	}
	defer func() {
		// save unless incident is new and closed (log alert)
		if incident != nil && (incident.Id != 0 || incident.Open) {
			_, err = data.UpdateIncidentState(incident)
		} else {
			err = data.SetUnevaluated(ak, event.Unevaluated) // if nothing to save, at least store the unevaluated state
		}
	}()
	// If nothing is out of the ordinary we are done
	if event.Status <= models.StNormal && incident == nil {
		return
	}

	// if event is unevaluated, we are done also.
	if incident != nil {
		incident.Unevaluated = event.Unevaluated
	}
	if event.Unevaluated {
		return
	}

	shouldNotify := false
	newIncident := false
	if incident == nil {
		incident = NewIncident(ak)
		newIncident = true
		shouldNotify = true
	}
	// set state.Result according to event result
	if event.Status == models.StCritical {
		incident.Result = event.Crit
	} else if event.Status == models.StWarning {
		incident.Result = event.Warn
	}

	if event.Status > models.StNormal {
		incident.LastAbnormalStatus = event.Status
		incident.LastAbnormalTime = event.Time.UTC().Unix()
	}
	if event.Status > incident.WorstStatus {
		incident.WorstStatus = event.Status
		shouldNotify = true
	}
	if event.Status != incident.CurrentStatus {
		incident.Events = append(incident.Events, *event)
	}
	incident.CurrentStatus = event.Status

	//run a preliminary save on new incidents to get an id
	if newIncident {
		if a.Log || silencedOrIgnored(a, event, si) {
			//a log or silenced/ignored alert will not need to be saved
		} else {
			incident.Id, err = s.DataAccess.State().UpdateIncidentState(incident)
			if err != nil {
				return
			}
		}
	}

	//render templates and open alert key if abnormal
	if event.Status > models.StNormal {
		s.executeTemplates(incident, event, a, r)
		incident.Open = true
		if a.Log {
			incident.Open = false
		}
	}

	// On state increase, clear old notifications and notify current.
	// Do nothing if state did not change.
	notify := func(ns *conf.Notifications) {
		if a.Log {
			lastLogTime := s.lastLogTimes[ak]
			now := utcNow()
			if now.Before(lastLogTime.Add(a.MaxLogFrequency)) {
				return
			}
			s.lastLogTimes[ak] = now
		}
		nots := ns.Get(s.Conf, incident.AlertKey.Group())
		for _, n := range nots {
			s.Notify(incident, n)
			checkNotify = true
		}
	}

	notifyCurrent := func() {
		//Auto close ignoreUnknowns for new incident.
		if silencedOrIgnored(a, event, si) {
			incident.Open = false
			return
		}
		incident.NeedAck = true
		switch event.Status {
		case models.StCritical, models.StUnknown:
			notify(a.CritNotification)
		case models.StWarning:
			notify(a.WarnNotification)
		}
	}

	// lock while we change notifications.
	s.Lock("RunHistory")
	if shouldNotify {
		incident.NeedAck = false
		if err = s.DataAccess.Notifications().ClearNotifications(ak); err != nil {
			return
		}
		notifyCurrent()
	}

	// finally close an open alert with silence once it goes back to normal.
	if si := silenced(ak); si != nil && event.Status == models.StNormal {
		go func(ak models.AlertKey) {
			slog.Infof("auto close %s because was silenced", ak)
			err := s.ActionByAlertKey("bosun", "Auto close because was silenced.", models.ActionClose, ak)
			if err != nil {
				slog.Errorln(err)
			}
		}(ak)
	}
	s.Unlock()
	return checkNotify, nil
}

func silencedOrIgnored(a *conf.Alert, event *models.Event, si *models.Silence) bool {
	if a.IgnoreUnknown && event.Status == models.StUnknown {
		return true
	}
	if si != nil && si.Forget && event.Status == models.StUnknown {
		return true
	}
	return false
}
func (s *Schedule) executeTemplates(state *models.IncidentState, event *models.Event, a *conf.Alert, r *RunHistory) {
	if event.Status != models.StUnknown {
		var errs []error
		metric := "template.render"
		//Render subject
		endTiming := collect.StartTimer(metric, opentsdb.TagSet{"alert": a.Name, "type": "subject"})
		subject, err := s.ExecuteSubject(r, a, state, false)
		if err != nil {
			slog.Infof("%s: %v", state.AlertKey, err)
			errs = append(errs, err)
		} else if subject == nil {
			err = fmt.Errorf("Empty subject on %s", state.AlertKey)
			slog.Error(err)
			errs = append(errs, err)
		}
		endTiming()

		//Render body
		endTiming = collect.StartTimer(metric, opentsdb.TagSet{"alert": a.Name, "type": "body"})
		body, _, err := s.ExecuteBody(r, a, state, false)
		if err != nil {
			slog.Infof("%s: %v", state.AlertKey, err)
			errs = append(errs, err)
		} else if subject == nil {
			err = fmt.Errorf("Empty body on %s", state.AlertKey)
			slog.Error(err)
			errs = append(errs, err)
		}
		endTiming()

		//Render email body
		endTiming = collect.StartTimer(metric, opentsdb.TagSet{"alert": a.Name, "type": "emailbody"})
		emailbody, attachments, err := s.ExecuteBody(r, a, state, true)
		if err != nil {
			slog.Infof("%s: %v", state.AlertKey, err)
			errs = append(errs, err)
		} else if subject == nil {
			err = fmt.Errorf("Empty email body on %s", state.AlertKey)
			slog.Error(err)
			errs = append(errs, err)
		}
		endTiming()

		//Render email subject
		endTiming = collect.StartTimer(metric, opentsdb.TagSet{"alert": a.Name, "type": "emailsubject"})
		emailsubject, err := s.ExecuteSubject(r, a, state, true)
		if err != nil {
			slog.Infof("%s: %v", state.AlertKey, err)
			errs = append(errs, err)
		} else if subject == nil {
			err = fmt.Errorf("Empty email subject on %s", state.AlertKey)
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
		//don't save email seperately if they are identical
		if string(state.EmailBody) != state.Body {
			state.EmailBody = emailbody
		}
		if string(state.EmailSubject) != state.Subject {
			state.EmailSubject = emailsubject
		}
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
	for _, alert := range s.Conf.GetAlerts() {
		severityCounts[alert.Name] = make(map[string]int64)
		abnormalCounts[alert.Name] = make(map[string]int64)
		var i models.Status
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
	for notificationName := range s.Conf.GetNotifications() {
		unAckOldestByNotification[notificationName] = time.Unix(1<<63-62135596801, 999999999)
		ackByNotificationCounts[notificationName] = make(map[bool]int64)
		ackByNotificationCounts[notificationName][false] = 0
		ackByNotificationCounts[notificationName][true] = 0
	}
	//TODO:
	//	for _, state := range s.status {
	//		if !state.Open {
	//			continue
	//		}
	//		name := state.AlertKey.Name()
	//		alertDef := s.Conf.Alerts[name]
	//		nots := make(map[string]bool)
	//		for name := range alertDef.WarnNotification.Get(s.Conf, state.Group) {
	//			nots[name] = true
	//		}
	//		for name := range alertDef.CritNotification.Get(s.Conf, state.Group) {
	//			nots[name] = true
	//		}
	//		incident, err := s.GetIncident(state.Last().IncidentId)
	//		if err != nil {
	//			slog.Errorln(err)
	//		}
	//		for notificationName := range nots {
	//			ackByNotificationCounts[notificationName][state.NeedAck]++
	//			if incident != nil && incident.Start.Before(unAckOldestByNotification[notificationName]) && state.NeedAck {
	//				unAckOldestByNotification[notificationName] = incident.Start
	//			}
	//		}
	//		severity := state.CurrentStatus.String()
	//		lastAbnormal := state.LastAbnormalStatus.String()
	//		severityCounts[state.Alert][severity]++
	//		abnormalCounts[state.Alert][lastAbnormal]++
	//		ackStatusCounts[state.Alert][state.NeedAck]++
	//		activeStatusCounts[state.Alert][state.IsActive()]++
	//	}
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
			ago = utcNow().Sub(timeStamp)
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

func (s *Schedule) GetUnknownAndUnevaluatedAlertKeys(alert string) (unknown, uneval []models.AlertKey) {
	unknown, uneval, err := s.DataAccess.State().GetUnknownAndUnevalAlertKeys(alert)
	if err != nil {
		slog.Errorf("Error getting unknown/unevaluated alert keys: %s", err)
		return nil, nil
	}
	return unknown, uneval
}

var bosunStartupTime = utcNow()

func (s *Schedule) findUnknownAlerts(now time.Time, alert string) []models.AlertKey {
	keys := []models.AlertKey{}
	if utcNow().Sub(bosunStartupTime) < s.Conf.GetCheckFrequency() {
		return keys
	}
	if !s.AlertSuccessful(alert) {
		return keys
	}
	a := s.Conf.GetAlert(alert)
	t := a.Unknown
	if t == 0 {
		t = s.Conf.GetCheckFrequency() * 2 * time.Duration(a.RunEvery)
	}
	maxTouched := now.UTC().Unix() - int64(t.Seconds())
	untouched, err := s.DataAccess.State().GetUntouchedSince(alert, maxTouched)
	if err != nil {
		slog.Errorf("Error finding unknown alerts for alert %s: %s.", alert, err)
		return keys
	}
	for _, ak := range untouched {
		if a.Squelch.Squelched(ak.Group()) {
			continue
		}
		keys = append(keys, ak)
	}
	return keys
}

func (s *Schedule) CheckAlert(T miniprofiler.Timer, r *RunHistory, a *conf.Alert) (cancelled bool) {
	slog.Infof("check alert %v start", a.Name)
	start := utcNow()
	for _, ak := range s.findUnknownAlerts(r.Start, a.Name) {
		r.Events[ak] = &models.Event{Status: models.StUnknown}
	}
	var warns, crits models.AlertKeys
	type res struct {
		results *expr.Results
		error   error
	}
	rc := make(chan res)
	var d *expr.Results
	var err error
	go func() {
		d, err := s.executeExpr(T, r, a, a.Depends)
		rc <- res{d, err}
	}()
	select {
	case res := <-rc:
		d = res.results
		err = res.error
	case <-s.runnerContext.Done():
		return true
	}
	var deps expr.ResultSlice
	if err == nil {
		deps = filterDependencyResults(d)
		crits, err, cancelled = s.CheckExpr(T, r, a, a.Crit, models.StCritical, nil)
		if err == nil && !cancelled {
			warns, err, cancelled = s.CheckExpr(T, r, a, a.Warn, models.StWarning, crits)
		}
	}
	if cancelled {
		return true
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
	return false
}

func removeUnknownEvents(evs map[models.AlertKey]*models.Event, alert string) {
	for k, v := range evs {
		if v.Status == models.StUnknown && k.Name() == alert {
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

func markDependenciesUnevaluated(events map[models.AlertKey]*models.Event, deps expr.ResultSlice, alert string) (unevalCount, unknownCount int) {
	for ak, ev := range events {
		if ak.Name() != alert {
			continue
		}
		for _, dep := range deps {
			if dep.Group.Overlaps(ak.Group()) {
				ev.Unevaluated = true
				unevalCount++
			}
			if ev.Status == models.StUnknown {
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
	providers := &expr.BosunProviders{
		Cache:     rh.Cache,
		Search:    s.Search,
		Squelched: s.Conf.AlertSquelched(a),
		History:   s,
	}
	results, _, err := e.Execute(rh.Backends, providers, T, rh.Start, 0, a.UnjoinedOK)
	return results, err
}

func (s *Schedule) CheckExpr(T miniprofiler.Timer, rh *RunHistory, a *conf.Alert, e *expr.Expr, checkStatus models.Status, ignore models.AlertKeys) (alerts models.AlertKeys, err error, cancelled bool) {
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
	type res struct {
		results *expr.Results
		error   error
	}
	rc := make(chan res)
	var results *expr.Results
	go func() {
		results, err := s.executeExpr(T, rh, a, e)
		rc <- res{results, err}
	}()
	select {
	case res := <-rc:
		results = res.results
		err = res.error
	case <-s.runnerContext.Done():
		return nil, nil, true
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
		n, err = valueToFloat(r.Value)
		if err != nil {
			return
		}
		event := rh.Events[ak]
		if event == nil {
			event = new(models.Event)
			rh.Events[ak] = event
		}
		result := &models.Result{
			Computations: r.Computations,
			Value:        models.Float(n),
			Expr:         e.String(),
		}
		switch checkStatus {
		case models.StWarning:
			event.Warn = result
		case models.StCritical:
			event.Crit = result
		}
		status := checkStatus
		if math.IsNaN(n) {
			status = checkStatus
		} else if n == 0 {
			status = models.StNormal
		}
		if status != models.StNormal {
			alerts = append(alerts, ak)
		}
		if status > rh.Events[ak].Status {
			event.Status = status
		}
	}
	return
}

func valueToFloat(val expr.Value) (float64, error) {
	var n float64
	switch v := val.(type) {
	case expr.Number:
		n = float64(v)
	case expr.Scalar:
		n = float64(v)
	default:
		return 0, fmt.Errorf("expected number or scalar")
	}
	return n, nil
}
