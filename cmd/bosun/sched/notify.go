package sched

import (
	"bytes"
	"fmt"
	htemplate "html/template"
	"strings"
	ttemplate "text/template"
	"time"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/models"
	"bosun.org/slog"
)

func (s *Schedule) dispatchNotifications() {
	ticker := time.NewTicker(s.Conf.CheckFrequency * 2)
	nextScheduled := time.After(s.CheckNotifications())
	for {
		select {
		case <-nextScheduled:
			nextScheduled = time.After(s.CheckNotifications())
		case <-s.nc:
			nextScheduled = time.After(s.CheckNotifications())
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

// CheckNotifications processes past notification events. It returns the
// duration until the soonest notification triggers.
func (s *Schedule) CheckNotifications() time.Duration {
	silenced := s.Silenced()
	s.Lock("CheckNotifications")
	defer s.Unlock()
	notifications := s.Notifications
	s.Notifications = nil
	for ak, ns := range notifications {
		if si := silenced(ak); si != nil {
			slog.Infoln("silencing", ak)
			continue
		}
		for name, t := range ns {
			n, present := s.Conf.Notifications[name]
			if !present {
				continue
			}
			remaining := t.Add(n.Timeout).Sub(time.Now())
			if remaining > 0 {
				s.AddNotification(ak, n, t)
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
				s.AddNotification(ak, n, t)
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
	timeout := time.Hour
	now := time.Now()
	for _, ns := range s.Notifications {
		for name, t := range ns {
			n, present := s.Conf.Notifications[name]
			if !present {
				continue
			}
			remaining := t.Add(n.Timeout).Sub(now)
			if remaining < timeout {
				timeout = remaining
			}
		}
	}
	return timeout
}

func (s *Schedule) sendNotifications(silenced SilenceTester) {
	if s.Conf.Quiet {
		slog.Infoln("quiet mode prevented", len(s.pendingNotifications), "notifications")
		return
	}
	for n, states := range s.pendingNotifications {
		for _, st := range states {
			ak := st.AlertKey
			silenced := silenced(ak) != nil
			if st.CurrentStatus == models.StUnknown {
				if silenced {
					slog.Infoln("silencing unknown", ak)
					continue
				}
				s.pendingUnknowns[n] = append(s.pendingUnknowns[n], st)
			} else if silenced {
				slog.Infoln("silencing", ak)
			} else {
				s.notify(st, n)
			}
			if n.Next != nil {
				s.AddNotification(ak, n.Next, time.Now().UTC())
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
		groupSets := ustates.GroupSets(s.Conf.MinGroupSize)
		for name, group := range groupSets {
			c++
			if c >= s.Conf.UnknownThreshold && s.Conf.UnknownThreshold > 0 {
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

var unknownMultiGroup = ttemplate.Must(ttemplate.New("unknownMultiGroup").Parse(`
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

func (s *Schedule) notify(st *models.IncidentState, n *conf.Notification) {
	n.Notify(st.Subject, st.Body, st.EmailSubject, st.EmailBody, s.Conf, string(st.AlertKey), st.Attachments...)
}

// utnotify is single notification for N unknown groups into a single notification
func (s *Schedule) utnotify(groups map[string]models.AlertKeys, n *conf.Notification) {
	var total int
	now := time.Now().UTC()
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
		s.Conf.UnknownThreshold,
	}); err != nil {
		slog.Errorln(err)
	}
	n.Notify(subject, body.String(), []byte(subject), body.Bytes(), s.Conf, "unknown_treshold")
}

var defaultUnknownTemplate = &conf.Template{
	Body: htemplate.Must(htemplate.New("").Parse(`
		<p>Time: {{.Time}}
		<p>Name: {{.Name}}
		<p>Alerts:
		{{range .Group}}
			<br>{{.}}
		{{end}}
	`)),
	Subject: ttemplate.Must(ttemplate.New("").Parse(`{{.Name}}: {{.Group | len}} unknown alerts`)),
}

func (s *Schedule) unotify(name string, group models.AlertKeys, n *conf.Notification) {
	subject := new(bytes.Buffer)
	body := new(bytes.Buffer)
	now := time.Now().UTC()
	s.Group[now] = group
	t := s.Conf.UnknownTemplate
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
	n.Notify(subject.String(), body.String(), subject.Bytes(), body.Bytes(), s.Conf, name)
}

func (s *Schedule) AddNotification(ak models.AlertKey, n *conf.Notification, started time.Time) {
	if s.Notifications == nil {
		s.Notifications = make(map[models.AlertKey]map[string]time.Time)
	}
	if s.Notifications[ak] == nil {
		s.Notifications[ak] = make(map[string]time.Time)
	}
	s.Notifications[ak][n.Name] = started
}

var actionNotificationSubjectTemplate *ttemplate.Template
var actionNotificationBodyTemplate *htemplate.Template

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
	actionNotificationSubjectTemplate = ttemplate.Must(ttemplate.New("").Parse(strings.Replace(subject, "\n", "", -1)))
	actionNotificationBodyTemplate = htemplate.Must(htemplate.New("").Parse(body))
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

		buf := &bytes.Buffer{}
		err := actionNotificationSubjectTemplate.Execute(buf, data)
		if err != nil {
			slog.Error("Error rendering action notification subject", err)
		}
		subject := buf.String()

		buf = &bytes.Buffer{}
		err = actionNotificationBodyTemplate.Execute(buf, data)
		if err != nil {
			slog.Error("Error rendering action notification body", err)
		}

		notification.Notify(subject, buf.String(), []byte(subject), buf.Bytes(), s.Conf, "actionNotification")
	}
	return nil
}

func (s *Schedule) groupActionNotifications(aks []models.AlertKey) (map[*conf.Notification][]*models.IncidentState, error) {
	groupings := make(map[*conf.Notification][]*models.IncidentState)
	for _, ak := range aks {
		alert := s.Conf.Alerts[ak.Name()]
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
		nots := n.Get(s.Conf, ak.Group())
		for _, not := range nots {
			if !not.RunOnActions {
				continue
			}
			groupings[not] = append(groupings[not], status)
		}
	}
	return groupings, nil
}
