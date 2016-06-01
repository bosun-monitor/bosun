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
	LastAbnormalTime       int64
	Unevaluated            bool
	NeedAck                bool
	Silenced               bool
	WarnNotificationChains [][]string
	CritNotificationChains [][]string
}

func MakeIncidentSummary(c *conf.Conf, s SilenceTester, is *models.IncidentState) IncidentSummaryView {
    warnNotifications := c.Alerts[is.AlertKey.Name()].WarnNotification.Get(c, is.AlertKey.Group())
    critNotifications := c.Alerts[is.AlertKey.Name()].CritNotification.Get(c, is.AlertKey.Group())
	return IncidentSummaryView{
		Id:                     is.Id,
		Subject:                is.Subject,
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
		WarnNotificationChains: conf.GetNotificationChains(c, warnNotifications),
		CritNotificationChains: conf.GetNotificationChains(c, critNotifications),
	}
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
			for k, v := range is.Tags {
				if k == sp[0] && glob.Glob(sp[1], v) {
					return true, nil
				}
				return false, nil
			}
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
	case "silence":
		switch value {
		case "true":
			return is.Silenced == true, nil
		case "false":
			return is.Silenced == false, nil
		default:
			return false, fmt.Errorf("unknown %s value: %s", key, value)
		}
	case "start":
		var op string
		val := value
		if strings.HasPrefix(value, "<") {
			op = "<"
			val = strings.TrimLeft(value, op)
		}
		if strings.HasPrefix(value, ">") {
			op = ">"
			val = strings.TrimLeft(value, op)
		}
		d, err := opentsdb.ParseDuration(val)
		if err != nil {
			return false, err
		}
		startTime := time.Unix(is.Start, 0)
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
	}
	return false, nil
}
