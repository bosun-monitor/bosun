package sched

import (
	"time"

	"bosun.org/cmd/bosun/conf"
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
func (s *Schedule) Notify(st *models.IncidentState, rt *models.RenderedTemplates, n *conf.Notification) bool {
	it := &IncidentWithTemplates{
		IncidentState:     st,
		RenderedTemplates: rt,
	}
	if s.pendingNotifications == nil {
		s.pendingNotifications = make(map[*conf.Notification][]*IncidentWithTemplates)
	}
	s.pendingNotifications[n] = append(s.pendingNotifications[n], it)
	return st.SetNotified(n.Name)
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
				// look at it again in a minute
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
			if s.Notify(st, rt, n) {
				_, err = s.DataAccess.State().UpdateIncidentState(st)
				if err != nil {
					slog.Error(err)
					continue
				}
			}
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
				gk := notificationGroupKey{notification: n, template: alert.Template}
				s.pendingUnknowns[gk] = append(s.pendingUnknowns[gk], st.IncidentState)
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
				s.QueueNotification(ak, n.Next, utcNow().Add(n.Timeout))
			}
		}
	}

}

// sendUnknownNotifications processes the schedule's pendingUnknowns queue. It puts unknowns into groups
// to be processed by the notification. When it is done processing the pendingUnknowns queue,
// it reinitializes the queue. Will send a maximum of $Unknown_Threshold notifications. If more are needed,
// the last one will be a multi-group.
func (s *Schedule) sendUnknownNotifications() {
	if len(s.pendingUnknowns) > 0 {
		slog.Info("Batching and sending unknown notifications")
		defer slog.Info("Done sending unknown notifications")
	}
	for gk, states := range s.pendingUnknowns {
		n := gk.notification
		ustates := make(States)
		for _, st := range states {
			ustates[st.AlertKey] = st
		}
		var c int
		var multiUstates []*models.IncidentState

		hitThreshold := false
		overThresholdSets := make(map[string]models.AlertKeys)
		minGroupSize := s.SystemConf.GetMinGroupSize()
		if n.UnknownMinGroupSize != nil {
			minGroupSize = *n.UnknownMinGroupSize
		}
		groupSets := ustates.GroupSets(minGroupSize)
		threshold := s.SystemConf.GetUnknownThreshold()
		if n.UnknownThreshold != nil {
			threshold = *n.UnknownThreshold
		}
		for name, group := range groupSets {
			c++
			for _, ak := range group {
				if c >= threshold && threshold > 0 {
					if !hitThreshold && len(groupSets) == c {
						// If the threshold is hit but only 1 email remains, just send the normal unknown
						n.NotifyUnknown(gk.template, s.SystemConf, name, group, ustates[ak])
						break
					}
					hitThreshold = true
					overThresholdSets[name] = group
					multiUstates = append(multiUstates, ustates[ak])
				} else {
					n.NotifyUnknown(gk.template, s.SystemConf, name, group, ustates[ak])
				}
			}
		}
		if len(overThresholdSets) > 0 {
			n.NotifyMultipleUnknowns(gk.template, s.SystemConf, overThresholdSets, multiUstates)
		}
	}
	s.pendingUnknowns = make(map[notificationGroupKey][]*models.IncidentState)
}

// notify is a wrapper for the notifications Notify method that sets the EmailSubject and EmailBody for the rendered
// template. It passes properties from the schedule that the Notification's Notify method requires.
func (s *Schedule) notify(st *models.IncidentState, rt *models.RenderedTemplates, n *conf.Notification) {
	n.NotifyAlert(rt, s.SystemConf, string(st.AlertKey), rt.Attachments...)
}

// QueueNotification persists a notification to the datastore to be sent in the future. This happens when
// there are notification chains or an alert is unevaluated due to a dependency.
func (s *Schedule) QueueNotification(ak models.AlertKey, n *conf.Notification, time time.Time) error {
	return s.DataAccess.Notifications().InsertNotification(ak, n.Name, time)
}

func (s *Schedule) ActionNotify(at models.ActionType, user, message string, aks []models.AlertKey) error {
	groupings, err := s.groupActionNotifications(at, aks)
	if err != nil {
		return err
	}
	for groupKey, states := range groupings {
		not := groupKey.notification
		if not.GroupActions == false {
			for _, state := range states {
				not.NotifyAction(at, groupKey.template, s.SystemConf, []*models.IncidentState{state}, user, message, s.RuleConf)
			}
		} else {
			incidents := []*models.IncidentState{}
			for _, state := range states {
				incidents = append(incidents, state)
			}
			not.NotifyAction(at, groupKey.template, s.SystemConf, incidents, user, message, s.RuleConf)
		}
	}
	return nil
}

// used to group notifications together. Notification alone is not sufficient, since different alerts
// can reference different templates.
// TODO: This may be overly aggressive at splitting things up. We really only need to seperate them if the
// specific keys referenced in the notification for action/unknown things are different between templates.
type notificationGroupKey struct {
	notification *conf.Notification
	template     *conf.Template
}

// group by notification and template
func (s *Schedule) groupActionNotifications(at models.ActionType, aks []models.AlertKey) (map[notificationGroupKey][]*models.IncidentState, error) {
	groupings := make(map[notificationGroupKey][]*models.IncidentState)
	for _, ak := range aks {
		alert := s.RuleConf.GetAlert(ak.Name())
		tmpl := alert.Template
		status, err := s.DataAccess.State().GetLatestIncident(ak)
		if err != nil {
			return nil, err
		}
		if alert == nil || status == nil {
			continue
		}
		// new way: incident keeps track of which notifications it has alerted.
		nots := map[string]*conf.Notification{}
		for _, name := range status.Notifications {
			not := s.RuleConf.GetNotification(name)
			if not != nil {
				nots[name] = not
			}
		}
		if len(nots) == 0 {
			// legacy behavior. Infer notifications from conf:
			var n *conf.Notifications
			if status.WorstStatus == models.StWarning || alert.CritNotification == nil {
				n = alert.WarnNotification
			} else {
				n = alert.CritNotification
			}
			if n == nil {
				continue
			}
			nots = n.Get(s.RuleConf, ak.Group())
		}
		for _, not := range nots {
			if !not.RunOnActionType(at) {
				continue
			}
			key := notificationGroupKey{not, tmpl}
			groupings[key] = append(groupings[key], status)
		}
	}
	return groupings, nil
}
