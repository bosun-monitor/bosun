package web

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/StackExchange/tsaf/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/tsaf/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/expr"
	"github.com/StackExchange/tsaf/sched"
)

func Expr(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	e, err := expr.New(r.FormValue("q"))
	if err != nil {
		return nil, err
	}
	res, queries, err := e.Execute(opentsdb.NewCache(schedule.Conf.TsdbHost, schedule.Conf.ResponseLimit), t)
	if err != nil {
		return nil, err
	}
	ret := struct {
		Type    string
		Results []*expr.Result
		Queries map[string]opentsdb.Request
	}{
		e.Tree.Root.Return().String(),
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
	}`, schedule.Conf.TsdbHost, r.FormValue("rule"))
	c, err := conf.New("-", txt)
	if err != nil {
		return nil, err
	}
	a := c.Alerts["_"]
	if a == nil || a.Crit == nil {
		return nil, fmt.Errorf("missing crit expression")
	}
	now := time.Now().UTC()
	if fd := r.FormValue("date"); len(fd) > 0 {
		if ft := r.FormValue("time"); len(ft) > 0 {
			fd += " " + ft
		} else {
			fd += " " + now.Format("15:04")
		}
		if t, err := time.Parse("2006-01-02 15:04", fd); err == nil {
			now = t
		} else {
			return nil, err
		}
	}
	all, queries, err := a.Crit.ExecuteOpts(opentsdb.NewCache(schedule.Conf.TsdbHost, schedule.Conf.ResponseLimit), t, now, 0)
	if err != nil {
		return nil, err
	}
	var res []*expr.Result
	for _, r := range all {
		if a.Squelched(r.Group) {
			continue
		}
		res = append(res, r)
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

type ResStatus struct {
	expr.Result
	Status string
	key    sched.AlertKey
}

type ResStatuses []ResStatus

func (a ResStatuses) Len() int           { return len(a) }
func (a ResStatuses) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ResStatuses) Less(i, j int) bool { return a[i].key < a[j].key }

func ToResult(s *sched.Schedule, ak sched.AlertKey) expr.Result {
	status := s.Status(ak)
	//TODO Get value in a better way, don't blow up if computations len is 0
	value := status.Computations[len(status.Computations)-1].Value
	return expr.Result{status.Computations, value, status.Group}
}

func Template(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var notifications string
	for _, v := range schedule.Conf.Notifications {
		notifications += "\n" + v.Def + "\n"
	}
	var vars string
	for k, v := range schedule.Conf.Vars {
		if strings.HasPrefix(k, "$") {
			vars += "\n" + k + "=" + v + "\n"
		}
	}
	txt := fmt.Sprintf(`
		tsdbHost = %s
		smtpHost = %s
		emailFrom = %s

		%s

		%s

		%s

		%s`, schedule.Conf.TsdbHost, schedule.Conf.SmtpHost, schedule.Conf.EmailFrom, vars,
		r.FormValue("template"), notifications, r.FormValue("alert"))
	c, err := conf.New("Test Config", txt)
	if err != nil {
		return nil, err
	}
	if len(c.Alerts) != 1 {
		return nil, fmt.Errorf("exactly one alert must be defined")
	}
	var a *conf.Alert
	for k, _ := range c.Alerts {
		a = c.Alerts[k]
		break
	}
	s := &sched.Schedule{}
	now := time.Now().UTC()
	if fd := r.FormValue("date"); len(fd) > 0 {
		if ft := r.FormValue("time"); len(ft) > 0 {
			fd += " " + ft
		} else {
			fd += " " + now.Format("15:04")
		}
		if t, err := time.Parse("2006-01-02 15:04", fd); err == nil {
			now = t
		} else {
			return nil, err
		}
	}
	s.CheckStart = now
	s.Load(c)
	critks, noncritks, err := s.CheckExpr(a, a.Crit, 0, nil)
	if err != nil {
		return nil, err
	}
	warnks, nonwarnks, err := s.CheckExpr(a, a.Warn, 0, critks)
	if err != nil {
		return nil, err
	}
	var res ResStatuses
	for _, v := range critks {
		res = append(res, ResStatus{ToResult(s, v), "Crit", v})
	}
	for _, v := range warnks {
		res = append(res, ResStatus{ToResult(s, v), "Warn", v})
	}
	okset := make(map[sched.AlertKey]bool)
	for _, v := range append(noncritks, nonwarnks...) {
		okset[v] = true
	}
	for _, v := range append(critks, warnks...) {
		delete(okset, v)
	}
	for k, _ := range okset {
		res = append(res, ResStatus{ToResult(s, k), "Ok", k})
	}
	if len(res) < 1 {
		return nil, fmt.Errorf("no results returned")
	}
	instance := s.Status(res[0].key)
	body := new(bytes.Buffer)
	subject := new(bytes.Buffer)
	if err := s.ExecuteBody(body, a, instance); err != nil {
		return nil, err
	}
	if err := s.ExecuteSubject(subject, a, instance); err != nil {
		return nil, err
	}
	b, _ := ioutil.ReadAll(body)
	sub, _ := ioutil.ReadAll(subject)
	sort.Sort(res)
	return struct {
		Body    string
		Subject string
		Result  ResStatuses
	}{
		string(b),
		string(sub),
		res,
	}, nil
}
