package web

import (
	"fmt"
	"net/http"
	"strings"

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
	Alert                  string
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
		return false, fmt.Errorf("bad ask length: sp is %v", sp)
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
	case "notify":
		//fmt.Println("notify", key, value)
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
	case "status": // CurrentStatus
		if is.CurrentStatus.String() == value {
			return true, nil
		}
		return false, nil
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
	// filter, err := sched.MakeFilter(r.FormValue("filter"))
	// if err != nil {
	// 	return nil, fmt.Errorf("bad filter: %v", err)
	// }
	parsedExpr, err := parse.Parse(r.FormValue("filter"))
	if err != nil {
		return nil, fmt.Errorf("bad filter: %v", err)
	}
	for _, iState := range list {
		// if !filter(schedule.Conf, schedule.Conf.Alerts[iState.Alert], iState) {
		// 	continue
		// }
		warnNotifications := schedule.Conf.Alerts[iState.AlertKey.Name()].WarnNotification.Get(schedule.Conf, iState.AlertKey.Group())
		critNotifications := schedule.Conf.Alerts[iState.AlertKey.Name()].CritNotification.Get(schedule.Conf, iState.AlertKey.Group())
		is := IncidentSummary{
			Id:                     iState.Id,
			Subject:                iState.Subject,
			Start:                  iState.Start.Unix(),
			Alert:                  iState.Alert,
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
