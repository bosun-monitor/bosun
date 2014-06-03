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
	now, err := getTime(r)
	if err != nil {
		return nil, err
	}
	res, queries, err := e.ExecuteOpts(opentsdb.NewCache(schedule.Conf.TsdbHost, schedule.Conf.ResponseLimit), t, now, 0)
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

func getTime(r *http.Request) (now time.Time, err error) {
	now = time.Now().UTC()
	if fd := r.FormValue("date"); len(fd) > 0 {
		if ft := r.FormValue("time"); len(ft) > 0 {
			fd += " " + ft
		} else {
			fd += " " + now.Format("15:04")
		}
		now, err = time.Parse("2006-01-02 15:04", fd)
	}
	return
}

type Res struct {
	*sched.Event
	Key sched.AlertKey
}

func Rule(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "tsdbHost = %s\n", schedule.Conf.TsdbHost)
	fmt.Fprintf(&buf, "smtpHost = %s\n", schedule.Conf.SmtpHost)
	fmt.Fprintf(&buf, "emailFrom = %s\n", schedule.Conf.EmailFrom)
	for k, v := range schedule.Conf.Vars {
		if strings.HasPrefix(k, "$") {
			fmt.Fprintf(&buf, "%s=%s\n", k, v)
		}
	}
	for _, v := range schedule.Conf.Notifications {
		fmt.Fprintln(&buf, v.Def)
	}
	fmt.Fprintf(&buf, "%s\n", r.FormValue("template"))
	fmt.Fprintf(&buf, "%s\n", r.FormValue("alert"))
	c, err := conf.New("Test Config", buf.String())
	if err != nil {
		return nil, err
	}
	if len(c.Alerts) != 1 {
		return nil, fmt.Errorf("exactly one alert must be defined")
	}
	s := &sched.Schedule{}
	now, err := getTime(r)
	if err != nil {
		return nil, err
	}
	s.CheckStart = now
	s.Init(c)
	var a *conf.Alert
	for _, a = range c.Alerts {
	}
	s.CheckExpr(a, a.Warn, sched.StWarning, nil)
	s.CheckExpr(a, a.Crit, sched.StCritical, nil)
	i := 0
	if len(s.RunHistory) < 1 {
		return nil, fmt.Errorf("no results returned")
	}
	keys := make(sched.AlertKeys, len(s.RunHistory))
	for k, v := range s.RunHistory {
		v.Time = now
		keys[i] = k
		i++
	}
	sort.Sort(keys)
	instance := s.Status(keys[0])
	body := new(bytes.Buffer)
	subject := new(bytes.Buffer)
	var warning []string
	if err := s.ExecuteBody(body, a, instance); err != nil {
		warning = append(warning, err.Error())
	}
	if err := s.ExecuteSubject(subject, a, instance); err != nil {
		warning = append(warning, err.Error())
	}
	b, _ := ioutil.ReadAll(body)
	sub, _ := ioutil.ReadAll(subject)
	return struct {
		Body    string
		Subject string
		Result  map[sched.AlertKey]*sched.Event
		Warning []string
		Time    int64
	}{
		string(b),
		string(sub),
		s.RunHistory,
		warning,
		now.Unix(),
	}, nil
}
