package web

import (
	"fmt"
	"net/http"
	"strings"

	"time"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/kylebrandt/boolq"
	"github.com/kylebrandt/boolq/parse"
	"github.com/ryanuber/go-glob"
)

type IncidentSummary struct {
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

func (is IncidentSummary) Ask(filter string) (bool, error) {
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

func ListOpenIncidents(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	// TODO: Retune this when we no longer store email bodies with incidents
	list, err := schedule.DataAccess.State().GetAllOpenIncidents()
	if err != nil {
		return nil, err
	}
	suppressor := schedule.Silenced()
	if suppressor == nil {
		return nil, fmt.Errorf("failed to get silences")
	}
	summaries := []IncidentSummary{}
	filterText := r.FormValue("filter")
	var parsedExpr *parse.Tree
	if filterText != "" {
		parsedExpr, err = parse.Parse(filterText)
		if err != nil {
			return nil, fmt.Errorf("bad filter: %v", err)
		}
	}
	for _, iState := range list {
		warnNotifications := schedule.Conf.Alerts[iState.AlertKey.Name()].WarnNotification.Get(schedule.Conf, iState.AlertKey.Group())
		critNotifications := schedule.Conf.Alerts[iState.AlertKey.Name()].CritNotification.Get(schedule.Conf, iState.AlertKey.Group())
		is := IncidentSummary{
			Id:                     iState.Id,
			Subject:                iState.Subject,
			Start:                  iState.Start.Unix(),
			AlertName:              iState.AlertKey.Name(),
			Tags:                   iState.AlertKey.Group(),
			TagsString:             iState.AlertKey.Group().String(),
			CurrentStatus:          iState.CurrentStatus,
			WorstStatus:            iState.WorstStatus,
			LastAbnormalStatus:     iState.LastAbnormalStatus,
			LastAbnormalTime:       iState.LastAbnormalTime,
			Unevaluated:            iState.Unevaluated,
			NeedAck:                iState.NeedAck,
			Silenced:               suppressor(iState.AlertKey) != nil,
			WarnNotificationChains: conf.GetNotificationChains(schedule.Conf, warnNotifications),
			CritNotificationChains: conf.GetNotificationChains(schedule.Conf, critNotifications),
		}
		if parsedExpr == nil {
			summaries = append(summaries, is)
			continue
		}
		match, err := boolq.AskParsedExpr(*parsedExpr, is)
		if err != nil {
			return nil, err
		}
		if match {
			summaries = append(summaries, is)
		}
	}
	return summaries, nil
}
