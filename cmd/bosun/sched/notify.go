package sched

import (
	htemplate "html/template"
	"strings"
	ttemplate "text/template"
	"time"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/models"
	"bosun.org/slog"
)

// Default hardcoded templates
var actionNotificationTemplate *conf.Template
var defaultUnknownMultigroupTemplate *conf.Template
var defaultUnknownTemplate *conf.Template

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

func (s *Schedule) Notify(st *models.IncidentState, n *conf.Notification) {
	if s.pendingNotifications == nil {
		s.pendingNotifications = make(map[*conf.Notification][]*models.IncidentState)
	}
	s.pendingNotifications[n] = append(s.pendingNotifications[n], st)
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
			s.Notify(st, n)
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
				s.pendingUnknowns[n] = append(s.pendingUnknowns[n], st)
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
				s.notify(st, n, alert)
			}
			if n.Next != nil {
				s.QueueNotification(ak, n.Next, utcNow())
			}
		}
	}
}

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

func (s *Schedule) notify(st *models.IncidentState, n *conf.Notification, a *conf.Alert) {
	msg := conf.NotificationMessage{
		Subject:     st.Subject,
		Body:        st.Body,
		Payload:     st.Payload,
		Attachments: st.Attachments,
		Vars:        make(map[string]interface{}),
	}

	msg.Vars["Alert"] = a
	msg.Vars["Incident"] = st
	msg.Vars["Notification"] = n

	n.Notify(msg, s.SystemConf, string(st.AlertKey))
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

	msg := defaultUnknownMultigroupTemplate.GenerateMessage(n, struct {
		Groups    map[string]models.AlertKeys
		Threshold int
	}{
		groups,
		s.SystemConf.GetUnknownThreshold(),
	})

	n.Notify(msg, s.SystemConf, "unknown_treshold")
}

func (s *Schedule) unotify(name string, group models.AlertKeys, n *conf.Notification) {
	now := utcNow()
	s.Group[now] = group
	t := s.RuleConf.GetUnknownTemplate()
	if t == nil {
		t = defaultUnknownTemplate
	}
	data := s.unknownData(now, name, group)

	msg := t.GenerateMessage(n, &data)

	n.Notify(msg, s.SystemConf, name)
}

func (s *Schedule) QueueNotification(ak models.AlertKey, n *conf.Notification, started time.Time) error {
	return s.DataAccess.Notifications().InsertNotification(ak, n.Name, started.Add(n.Timeout))
}

func init() {
	subject := `{{$first := index .States 0}}{{$count := len .States}}
{{.User}} {{.ActionType}}
{{if gt $count 1}} {{$count}} Alerts. 
{{else}} Incident #{{$first.Id}} ({{$first.Subject}}) 
{{end}}`
	body := `{{$count := len .States}}{{.User}} {{.ActionType}} {{$count}} alert{{if gt $count 1}}s{{end}}: <br/>
<strong>Message:</strong> {{.Message}} <br/>
<strong>Incidents:</strong> <br/>
<ul>
	{{range .States}}
		<li>
			<a href="{{$.IncidentLink .Id}}">#{{.Id}}:</a> 
			{{.Subject}}
		</li>
	{{end}}
</ul>`
	payload := `
{{$count := len .States}}{{.User}} {{.ActionType}} {{$count}} alert{{if gt $count 1}}s{{end}}:
Message: {{.Message}}
Incidents:
	{{range .States}}
			<a href="{{$.IncidentLink .Id}}">#{{.Id}}:</a>
			{{.Subject}}
	{{end}}
	`

	actionNotificationSubjectTemplate := ttemplate.Must(ttemplate.New("").Parse(strings.Replace(subject, "\n", "", -1)))
	actionNotificationBodyTemplate := htemplate.Must(htemplate.New("").Parse(body))
	actionNotificationPayloadTemplate := ttemplate.Must(ttemplate.New("").Parse(payload))

	actionNotificationTemplate = &conf.Template{
		Body:    actionNotificationBodyTemplate,
		Subject: actionNotificationSubjectTemplate,
		Payload: actionNotificationPayloadTemplate,
	}

	defaultUnknownMultigroupTemplate = &conf.Template{
		Subject: ttemplate.Must(ttemplate.New("").Parse(`{{.Name}}: {{.Group | len}} unknown alerts`)),
		Body: htemplate.Must(htemplate.New("unknownMultiGroup").Parse(`
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
	`)),
	}

	defaultUnknownTemplate = &conf.Template{
		Subject: ttemplate.Must(ttemplate.New("").Parse(`{{.Name}}: {{.Group | len}} unknown alerts`)),
		Body: htemplate.Must(htemplate.New("").Parse(`
		<p>Time: {{.Time}}
		<p>Name: {{.Name}}
		<p>Alerts:
		{{range .Group}}
			<br>{{.}}
		{{end}}
	`)),
	}
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

		msg := actionNotificationTemplate.GenerateMessage(notification, data)

		notification.Notify(msg, s.SystemConf, "actionNotification")
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
