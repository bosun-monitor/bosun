package sched

import (
	"fmt"
	"strings"
	"time"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"github.com/ryanuber/go-glob"
)

// Views

type EventSummary struct {
	Status models.Status
	Time   int64
}

// EventSummary is like a models.Event but strips the Results and Unevaluated
func MakeEventSummary(e models.Event) (EventSummary, bool) {
	return EventSummary{
		Status: e.Status,
		Time:   e.Time.Unix(),
	}, e.Unevaluated
}

type EpochAction struct {
	User    string
	Message string
	Time    int64
	Type    models.ActionType
}

func MakeEpochAction(a models.Action) EpochAction {
	return EpochAction{
		User:    a.User,
		Message: a.Message,
		Time:    a.Time.UTC().Unix(),
		Type:    a.Type,
	}
}

type IncidentSummaryView struct {
	Id                     int64
	Subject                string
	Start                  int64
	AlertName              string
	Tags                   opentsdb.TagSet
	TagsString             string
	CurrentStatus          models.Status
	WorstStatus            models.Status
	LastAbnormalStatus     models.Status
	LastAbnormalTime       models.Epoch
	Unevaluated            bool
	NeedAck                bool
	Silenced               bool
	Actions                []EpochAction
	Events                 []EventSummary
	WarnNotificationChains [][]string
	CritNotificationChains [][]string
	LastStatusTime         int64
}

func MakeIncidentSummary(c conf.RuleConfProvider, s SilenceTester, is *models.IncidentState) (*IncidentSummaryView, error) {
	alert := c.GetAlert(is.AlertKey.Name())
	if alert == nil {
		return nil, fmt.Errorf("alert %v does not exist in the configuration", is.AlertKey.Name())
	}
	warnNotifications := alert.WarnNotification.Get(c, is.AlertKey.Group())
	critNotifications := alert.CritNotification.Get(c, is.AlertKey.Group())
	eventSummaries := []EventSummary{}
	nonNormalNonUnknownCount := 0
	for _, event := range is.Events {
		if event.Status > models.StNormal && event.Status < models.StUnknown {
			nonNormalNonUnknownCount++
		}
		if eventSummary, unevaluated := MakeEventSummary(event); !unevaluated {
			eventSummaries = append(eventSummaries, eventSummary)
		}
	}
	actions := make([]EpochAction, len(is.Actions))
	for i, action := range is.Actions {
		actions[i] = MakeEpochAction(action)
	}
	subject := is.Subject
	// There is no rendered subject when the state is unknown and
	// there is no other non-normal status in the history.
	if subject == "" && nonNormalNonUnknownCount == 0 {
		subject = fmt.Sprintf("%s: %v", is.CurrentStatus, is.AlertKey)
	}
	return &IncidentSummaryView{
		Id:                     is.Id,
		Subject:                subject,
		Start:                  is.Start.Unix(),
		AlertName:              is.AlertKey.Name(),
		Tags:                   is.AlertKey.Group(),
		TagsString:             is.AlertKey.Group().String(),
		CurrentStatus:          is.CurrentStatus,
		WorstStatus:            is.WorstStatus,
		LastAbnormalStatus:     is.LastAbnormalStatus,
		LastAbnormalTime:       is.LastAbnormalTime,
		Unevaluated:            is.Unevaluated,
		NeedAck:                is.NeedAck,
		Silenced:               s(is.AlertKey) != nil,
		Actions:                actions,
		Events:                 eventSummaries,
		WarnNotificationChains: conf.GetNotificationChains(warnNotifications),
		CritNotificationChains: conf.GetNotificationChains(critNotifications),
		LastStatusTime:         is.Last().Time.Unix(),
	}, nil
}

func (is IncidentSummaryView) Ask(filter string) (bool, error) {
	sp := strings.SplitN(filter, ":", 2)
	if len(sp) != 2 {
		return false, fmt.Errorf("bad filter, filter must be in k:v format, got %v", filter)
	}
	key := sp[0]
	value := sp[1]
	switch key {
	case "ack":
		switch value {
		case "true":
			return is.NeedAck == false, nil
		case "false":
			return is.NeedAck == true, nil
		default:
			return false, fmt.Errorf("unknown %s value: %s", key, value)
		}
	case "ackTime":
		if is.NeedAck == true {
			return false, nil
		}
		for _, action := range is.Actions {
			if action.Type == models.ActionAcknowledge {
				return checkTimeArg(action.Time, value)
			}
		}
		// In case an incident does not need ack but have no ack event
		// I found one case of this from bosun autoclosing, but it was
		// a very old incident. Don't think we should end up here, but
		// if we do better to show the ack'd incident than hide it.
		return true, nil
	case "hasTag":
		if strings.Contains(value, "=") {
			if strings.HasPrefix(value, "=") {
				q := strings.TrimPrefix(value, "=")
				for _, v := range is.Tags {
					if glob.Glob(q, v) {
						return true, nil
					}
				}
				return false, nil
			}
			if strings.HasSuffix(value, "=") {
				q := strings.TrimSuffix(value, "=")
				_, ok := is.Tags[q]
				return ok, nil
			}
			sp := strings.Split(value, "=")
			if len(sp) != 2 {
				return false, fmt.Errorf("unexpected tag specification: %v", value)
			}
			tagValues := strings.Split(sp[1], "|")
			for k, v := range is.Tags {
				for _, tagValue := range tagValues {
					if k == sp[0] && glob.Glob(tagValue, v) {
						return true, nil
					}
				}
			}
			return false, nil
		}
		q := strings.TrimRight(value, "=")
		_, ok := is.Tags[q]
		return ok, nil
	case "hidden":
		hide := is.Silenced || is.Unevaluated
		switch value {
		case "true":
			return hide == true, nil
		case "false":
			return hide == false, nil
		default:
			return false, fmt.Errorf("unknown %s value: %s", key, value)
		}
	case "name":
		return glob.Glob(value, is.AlertName), nil
	case "user":
		for _, action := range is.Actions {
			if action.User == value {
				return true, nil
			}
		}
		return false, nil
	case "notify":
		for _, chain := range is.WarnNotificationChains {
			for _, wn := range chain {
				if glob.Glob(value, wn) {
					return true, nil
				}
			}
		}
		for _, chain := range is.CritNotificationChains {
			for _, cn := range chain {
				if glob.Glob(value, cn) {
					return true, nil
				}
			}
		}
		return false, nil
	case "silenced":
		switch value {
		case "true":
			return is.Silenced == true, nil
		case "false":
			return is.Silenced == false, nil
		default:
			return false, fmt.Errorf("unknown %s value: %s", key, value)
		}
	case "start":
		return checkTimeArg(is.Start, value)
	case "unevaluated":
		switch value {
		case "true":
			return is.Unevaluated == true, nil
		case "false":
			return is.Unevaluated == false, nil
		default:
			return false, fmt.Errorf("unknown %s value: %s", key, value)
		}
	case "status": // CurrentStatus
		return is.CurrentStatus.String() == value, nil
	case "worstStatus":
		return is.WorstStatus.String() == value, nil
	case "lastAbnormalStatus":
		return is.LastAbnormalStatus.String() == value, nil
	case "subject":
		return glob.Glob(value, is.Subject), nil
	case "since":
		return checkTimeArg(is.LastStatusTime, value)
	}
	return false, nil
}

func checkTimeArg(ts int64, arg string) (bool, error) {
	var op string
	val := arg
	if strings.HasPrefix(arg, "<") {
		op = "<"
		val = strings.TrimLeft(arg, op)
	}
	if strings.HasPrefix(arg, ">") {
		op = ">"
		val = strings.TrimLeft(arg, op)
	}
	d, err := opentsdb.ParseDuration(val)
	if err != nil {
		return false, err
	}
	startTime := time.Unix(ts, 0)
	// might want to make Now a property of incident summary for viewing things in the past
	// but not going there at the moment. This is because right now I'm working with open
	// incidents. And "What did incidents look like at this time?" is a different question
	// since those incidents will no longer be open.
	relativeTime := time.Now().UTC().Add(time.Duration(-d))
	switch op {
	case ">", "":
		return startTime.After(relativeTime), nil
	case "<":
		return startTime.Before(relativeTime), nil
	default:
		return false, fmt.Errorf("unexpected op: %v", op)
	}
}
