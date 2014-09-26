package web

import (
	"bytes"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/bosun/conf"
	"github.com/StackExchange/bosun/expr"
	"github.com/StackExchange/bosun/sched"
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
	res, queries, err := e.Execute(opentsdb.NewCache(schedule.Conf.TsdbHost, schedule.Conf.ResponseLimit), t, now, 0, false, schedule.Search, schedule.Lookups)
	if err != nil {
		return nil, err
	}
	for _, r := range res.Results {
		if r.Computations == nil {
			r.Computations = make(expr.Computations, 0)
		}
	}
	ret := struct {
		Type    string
		Results []*expr.Result
		Queries map[string]opentsdb.Request
	}{
		e.Tree.Root.Return().String(),
		res.Results,
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
	Key expr.AlertKey
}

func Rule(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "tsdbHost = %s\n", schedule.Conf.TsdbHost)
	fmt.Fprintf(&buf, "smtpHost = %s\n", schedule.Conf.SmtpHost)
	fmt.Fprintf(&buf, "emailFrom = %s\n", schedule.Conf.EmailFrom)
	fmt.Fprintf(&buf, "responseLimit = %d\n", schedule.Conf.ResponseLimit)
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
	s.Search = schedule.Search
	rh := make(sched.RunHistory)
	var a *conf.Alert
	for _, a = range c.Alerts {
	}
	if _, err := s.CheckExpr(rh, a, a.Warn, sched.StWarning, nil); err != nil {
		return nil, err
	}
	if _, err := s.CheckExpr(rh, a, a.Crit, sched.StCritical, nil); err != nil {
		return nil, err
	}
	i := 0
	if len(rh) < 1 {
		return nil, fmt.Errorf("no results returned")
	}
	keys := make(expr.AlertKeys, len(rh))
	for k, v := range rh {
		v.Time = now
		keys[i] = k
		i++
	}
	sort.Sort(keys)
	instance := s.Status(keys[0])
	instance.History = []sched.Event{*rh[keys[0]]}
	body := new(bytes.Buffer)
	subject := new(bytes.Buffer)
	var data interface{}
	warning := make([]string, 0)
	if r.FormValue("notemplate") != "" {
		if err := s.ExecuteBody(body, a, instance); err != nil {
			warning = append(warning, err.Error())
		}
		if err := s.ExecuteSubject(subject, a, instance); err != nil {
			warning = append(warning, err.Error())
		}
		data = s.Data(instance, a)
		if e := r.FormValue("email"); e != "" {
			n := conf.Notification{
				Email: []*mail.Address{&mail.Address{
					Name:    "Bosun Test",
					Address: e,
				}},
			}
			n.DoEmail(subject.Bytes(), body.Bytes(), schedule.Conf.EmailFrom, schedule.Conf.SmtpHost)
		}
	}
	return struct {
		Body    string      `json:",omitempty"`
		Subject string      `json:",omitempty"`
		Data    interface{} `json:",omitempty"`
		Result  sched.RunHistory
		Warning []string `json:",omitempty"`
		Time    int64
	}{
		body.String(),
		subject.String(),
		data,
		rh,
		warning,
		now.Unix(),
	}, nil
}
