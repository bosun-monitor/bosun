package web

import (
	"fmt"
	"net/http"

	"bosun.org/cmd/bosun/sched"

	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/kylebrandt/boolq"
)

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
	summaries := []sched.IncidentSummaryView{}
	filterText := r.FormValue("filter")
	var parsedExpr *boolq.Tree
	parsedExpr, err = boolq.Parse(filterText)
	if err != nil {
		return nil, fmt.Errorf("bad filter: %v", err)
	}
	for _, iState := range list {
		is := sched.MakeIncidentSummary(schedule.RuleConf, suppressor, iState)
		match, err := boolq.AskParsedExpr(parsedExpr, is)
		if err != nil {
			return nil, err
		}
		if match {
			summaries = append(summaries, is)
		}
	}
	return summaries, nil
}
