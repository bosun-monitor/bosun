package web

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/expr"
)

func Expr(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	e, err := expr.New(r.FormValue("q"))
	if err != nil {
		return nil, err
	}
	res, queries, err := e.Execute(opentsdb.Host(schedule.Conf.TsdbHost), t)
	if err != nil {
		return nil, err
	}
	ret := struct {
		Results []*expr.Result
		Queries map[string]opentsdb.Request
	}{
		res,
		make(map[string]opentsdb.Request),
	}
	for _, q := range queries {
		if e, err := url.QueryUnescape(q.String()); err == nil {
			ret.Queries[e] = q
		}
	}
	return ret, nil
}

func Rule(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	txt := fmt.Sprintf(`
	tsdbHost = %s
	alert _ {
		%s
	}`, schedule.Conf.TsdbHost, r.FormValue("q"))
	c, err := conf.New("-", txt)
	if err != nil {
		return nil, err
	}
	a := c.Alerts["_"]
	if a == nil || a.Crit == nil {
		return nil, fmt.Errorf("missing crit expression")
	}
	res, queries, err := a.Crit.Execute(opentsdb.Host(schedule.Conf.TsdbHost), t)
	if err != nil {
		return nil, err
	}
	ret := struct {
		Results []*expr.Result
		Queries map[string]opentsdb.Request
	}{
		res,
		make(map[string]opentsdb.Request),
	}
	for _, q := range queries {
		if e, err := url.QueryUnescape(q.String()); err == nil {
			ret.Queries[e] = q
		}
	}
	return ret, nil
}
