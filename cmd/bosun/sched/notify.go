package sched

import (
	"bytes"
	"fmt"
	"net/url"
	"time"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/conf/template"
	"bosun.org/models"
	"bosun.org/slog"
)

// dispatchNotifications triggers notification checks at 2x the the system configuration's
// check frequency, when something has signaled the schedule via the nc channels, or when
// a notification that was scheduled in the future due to a notification chain
func (s *Schedule) dispatchNotifications() {
	ticker := time.NewTicker(s.SystemConf.GetCheckFrequency() * 2)
	var next <-chan time.Time
	nextAt := func(t time.Time) {
		diff := t.Sub(utcNow())
		if diff <= 0 {
			diff = time.Millisecond
		}
		next = time.After(diff)
	}
	nextAt(utcNow())
	for {
		select {
		case <-next:
			nextAt(s.CheckNotifications())
		case <-s.nc:
			nextAt(s.CheckNotifications())
		case <-ticker.C:
			s.sendUnknownNotifications()
		}
	}

}

type IncidentWithTemplates struct {
	*models.IncidentState
	*models.RenderedTemplates
}

// Notify puts a rendered notification in the schedule's pendingNotifications queue
func (s *Schedule) Notify(st *models.IncidentState, rt *models.RenderedTemplates, n *conf.Notification) {
	it := &IncidentWithTemplates{}
	it.IncidentState = st
	it.RenderedTemplates = rt
	if s.pendingNotifications == nil {
		s.pendingNotifications = make(map[*conf.Notification][]*IncidentWithTemplates)
	}
	s.pendingNotifications[n] = append(s.pendingNotifications[n], it)
}

// CheckNotifications processes past notification events. It returns the next time a notification is needed.
func (s *Schedule) CheckNotifications() time.Time {
	silenced := s.Silenced()
	s.Lock("CheckNotifications")
	defer s.Unlock()
	latestTime := utcNow()
	notifications, err := s.DataAccess.Notifications().GetDueNotifications()
	if err != nil {
		slog.Error("Error getting notifications", err)
		return utcNow().Add(time.Minute)
	}
	for ak, ns := range notifications {
		if si := silenced(ak); si != nil {
			slog.Infoln("silencing", ak)
			continue
		}
		for name, t := range ns {
			n := s.RuleConf.GetNotification(name)
			if n == nil {
				continue
			}
			//If alert is currently unevaluated because of a dependency,
			//simply requeue it until the dependency resolves itself.
			_, uneval := s.GetUnknownAndUnevaluatedAlertKeys(ak.Name())
			unevaluated := false
			for _, un := range uneval {
				if un == ak {
					unevaluated = true
					break
				}
			}
			if unevaluated {
				s.QueueNotification(ak, n, t.Add(time.Minute))
				continue
			}
			st, err := s.DataAccess.State().GetLatestIncident(ak)

			if err != nil {
				slog.Error(err)
				continue
			}
			if st == nil {
				continue
			}
			rt, err := s.DataAccess.State().GetRenderedTemplates(st.Id)
			if err != nil {
				slog.Error(err)
				continue
			}
			s.Notify(st, rt, n)
		}
	}
	s.sendNotifications(silenced)
	s.pendingNotifications = nil
	err = s.DataAccess.Notifications().ClearNotificationsBefore(latestTime)
	if err != nil {
		slog.Error("Error clearing notifications", err)
		return utcNow().Add(time.Minute)
	}
	timeout, err := s.DataAccess.Notifications().GetNextNotificationTime()
	if err != nil {
		slog.Error("Error getting next notification time", err)
		return utcNow().Add(time.Minute)
	}
	return timeout
}

// sendNotifications processes the schedule's pendingNotifications queue. It silences notifications,
// moves unknown notifications to the unknownNotifications queue so they can be grouped, calls the notification
// Notify method to trigger notification actions, and queues notifications that are in the future because they
// are part of a notification chain
func (s *Schedule) sendNotifications(silenced SilenceTester) {
	if s.quiet {
		slog.Infoln("quiet mode prevented", len(s.pendingNotifications), "notifications")
		return
	}
	for n, states := range s.pendingNotifications {
		for _, st := range states {
			ak := st.AlertKey
			alert := s.RuleConf.GetAlert(ak.Name())
			if alert == nil {
				continue
			}
			silenced := silenced(ak) != nil
			if st.CurrentStatus == models.StUnknown {
				if silenced {
					slog.Infoln("silencing unknown", ak)
					continue
				}
				s.pendingUnknowns[n] = append(s.pendingUnknowns[n], st.IncidentState)
			} else if silenced {
				slog.Infof("silencing %s", ak)
				continue
			} else if !alert.Log && (!st.Open || !st.NeedAck) {
				slog.Errorf("Cannot notify acked or closed alert %s. Clearing.", ak)
				if err := s.DataAccess.Notifications().ClearNotifications(ak); err != nil {
					slog.Error(err)
				}
				continue
			} else {
				s.notify(st.IncidentState, st.RenderedTemplates, n)
			}
			if n.Next != nil {
				s.QueueNotification(ak, n.Next, utcNow())
			}
		}
	}
}

// sendUnknownNotifications processes the schedule's pendingUnknowns queue. It puts unknowns into groups
// to be processed by the schedule's utnotify method. When it is done processing the pendingUnknowns queue
// it reinitializes the queue.
func (s *Schedule) sendUnknownNotifications() {
	slog.Info("Batching and sending unknown notifications")
	defer slog.Info("Done sending unknown notifications")
	for n, states := range s.pendingUnknowns {
		ustates := make(States)
		for _, st := range states {
			ustates[st.AlertKey] = st
		}
		var c int
		tHit := false
		oTSets := make(map[string]models.AlertKeys)
		groupSets := ustates.GroupSets(s.SystemConf.GetMinGroupSize())
		for name, group := range groupSets {
			c++
			if c >= s.SystemConf.GetUnknownThreshold() && s.SystemConf.GetUnknownThreshold() > 0 {
				if !tHit && len(groupSets) == 0 {
					// If the threshold is hit but only 1 email remains, just send the normal unknown
					s.unotify(name, group, n)
					break
				}
				tHit = true
				oTSets[name] = group
			} else {
				s.unotify(name, group, n)
			}
		}
		if len(oTSets) > 0 {
			s.utnotify(oTSets, n)
		}
	}
	s.pendingUnknowns = make(map[*conf.Notification][]*models.IncidentState)
}

var unknownMultiGroup = template.Must(template.New("unknownMultiGroupHTML").Parse(`
	<p>Threshold of {{ .Threshold }} reached for unknown notifications. The following unknown
	group emails were not sent.
	<ul>
	{{ range $group, $alertKeys := .Groups }}
		<li>
			{{ $group }}
			<ul>
				{{ range $ak := $alertKeys }}
				<li>{{ $ak }}</li>
				{{ end }}
			<ul>
		</li>
	{{ end }}
	</ul>
	`))

// notify is a wrapper for the notifications Notify method that sets the EmailSubject and EmailBody for the rendered
// template. It passes properties from the schedule that the Notification's Notify method requires.
func (s *Schedule) notify(st *models.IncidentState, rt *models.RenderedTemplates, n *conf.Notification) {
	n.NotifyAlert(rt, s.SystemConf, string(st.AlertKey), rt.Attachments...)
}

// utnotify is single notification for N unknown groups into a single notification
func (s *Schedule) utnotify(groups map[string]models.AlertKeys, n *conf.Notification) {
	var total int
	now := utcNow()
	for _, group := range groups {
		// Don't know what the following line does, just copied from unotify
		s.Group[now] = group
		total += len(group)
	}
	subject := fmt.Sprintf("%v unknown alert instances suppressed", total)
	body := new(bytes.Buffer)
	if err := unknownMultiGroup.Execute(body, struct {
		Groups    map[string]models.AlertKeys
		Threshold int
	}{
		groups,
		s.SystemConf.GetUnknownThreshold(),
	}); err != nil {
		slog.Errorln(err)
	}
	rt := &models.RenderedTemplates{
		Subject: subject,
		Body:    body.String(),
	}
	// TODO: Fix this
	//n.Notify(rt, s.SystemConf, "unknown_threshold")
}

var defaultUnknownTemplate = &conf.Template{
	Body: template.Must(template.New("body").Parse(`
		<p>Time: {{.Time}}
		<p>Name: {{.Name}}
		<p>Alerts:
		{{range .Group}}
			<br>{{.}}
		{{end}}
	`)),
	Subject: template.Must(template.New("subject").Parse(`{{.Name}}: {{.Group | len}} unknown alerts`)),
}

// unotify builds an unknown notification for an alertkey or a group of alert keys. It renders the template
// and calls the notification's Notify method to trigger the action.
func (s *Schedule) unotify(name string, group models.AlertKeys, n *conf.Notification) {
	subject := new(bytes.Buffer)
	body := new(bytes.Buffer)
	now := utcNow()
	s.Group[now] = group
	t := s.RuleConf.GetUnknownTemplate()
	if t == nil {
		t = defaultUnknownTemplate
	}
	data := s.unknownData(now, name, group)
	if t.Body != nil {
		if err := t.Body.Execute(body, &data); err != nil {
			slog.Infoln("unknown template error:", err)
		}
	}
	if t.Subject != nil {
		if err := t.Subject.Execute(subject, &data); err != nil {
			slog.Infoln("unknown template error:", err)
		}
	}
	rt := &models.RenderedTemplates{
		Subject: subject.String(),
		Body:    body.String(),
	}
	// TODO: Unknown
	//n.Notify(rt, s.SystemConf, name)
}

// QueueNotification persists a notification to the datastore to be sent in the future. This happens when
// there are notification chains or an alert is unevaluated due to a dependency.
func (s *Schedule) QueueNotification(ak models.AlertKey, n *conf.Notification, started time.Time) error {
	return s.DataAccess.Notifications().InsertNotification(ak, n.Name, started.Add(n.Timeout))
}

type actionNotificationContext struct {
	States     []*models.IncidentState
	User       string
	Message    string
	ActionType models.ActionType

	schedule *Schedule
}

func (a actionNotificationContext) IncidentLink(i int64) string {
	return a.schedule.SystemConf.MakeLink("/incident", &url.Values{
		"id": []string{fmt.Sprint(i)},
	})
}

func (s *Schedule) ActionNotify(at models.ActionType, user, message string, aks []models.AlertKey) error {
	groupings, err := s.groupActionNotifications(aks)
	if err != nil {
		return err
	}
	for notification, states := range groupings {
		incidents := []*models.IncidentState{}
		for _, state := range states {
			incidents = append(incidents, state)
		}
		data := actionNotificationContext{incidents, user, message, at, s}
	}
	return nil
}

func (s *Schedule) groupActionNotifications(aks []models.AlertKey) (map[*conf.Notification][]*models.IncidentState, error) {
	groupings := make(map[*conf.Notification][]*models.IncidentState)
	for _, ak := range aks {
		alert := s.RuleConf.GetAlert(ak.Name())
		status, err := s.DataAccess.State().GetLatestIncident(ak)
		if err != nil {
			return nil, err
		}
		if alert == nil || status == nil {
			continue
		}
		var n *conf.Notifications
		if status.WorstStatus == models.StWarning || alert.CritNotification == nil {
			n = alert.WarnNotification
		} else {
			n = alert.CritNotification
		}
		if n == nil {
			continue
		}
		nots := n.Get(s.RuleConf, ak.Group())
		for _, not := range nots {
			if !not.RunOnActions {
				continue
			}
			groupings[not] = append(groupings[not], status)
		}
	}
	return groupings, nil
}
