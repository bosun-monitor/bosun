package web

import (
	"net/http"

	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/expr"
)

func Expr(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	e, err := expr.New(r.FormValue("q"))
	if err != nil {
		return nil, err
	}
	res, err := e.Execute(opentsdb.Host(schedule.Conf.TsdbHost), t)
	if err != nil {
		return nil, err
	}
	return res, nil
}
