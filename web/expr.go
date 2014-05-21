package web

import (
	"fmt"
	"net/http"
	"net/url"
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

type ResStatus struct {
	expr.Result
	Status string
	key    sched.AlertKey
}

func Rule(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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
	s.Init(c)
	s.RunChecks()
	for k, v := range s.RunHistory {
		fmt.Println(k, v)
	}
	return s.RunHistory, nil

	// critks, err := s.CheckExpr(a, a.Crit, 0, nil)
	// for _, v := range critks {
	// 	s.Status(v).Append(sched.StCritical)
	// }
	// if err != nil {
	// 	return nil, err
	// }
	// warnks, err := s.CheckExpr(a, a.Warn, 0, critks)
	// for _, v := range critks {
	// 	s.Status(v).Append(sched.StWarning)
	// }
	// if err != nil {
	// 	return nil, err
	// }
	// res := make(sched.States)
	// for _, v := range append(critks, warnks...) {
	// 	res[v] = s.Status(v)
	// }
	// for k, _ := range s.RunStates {
	// 	s.Status(k).Append(sched.StNormal)
	// 	res[k] = s.Status(k)
	// }
	// if len(res) < 1 {
	// 	return nil, fmt.Errorf("no results returned")
	// }
	// keys := make(sched.AlertKeys, len(res))
	// i := 0
	// for k, _ := range res {
	// 	fmt.Println(k)
	// 	keys[i] = k
	// 	i++
	// }
	// sort.Sort(keys)
	// fmt.Println(keys, res)
	// instance := res[keys[0]]
	// fmt.Println(instance)
	// body := new(bytes.Buffer)
	// subject := new(bytes.Buffer)
	// var warning []string
	// if err := s.ExecuteBody(body, a, instance); err != nil {
	// 	warning = append(warning, err.Error())
	// }
	// if err := s.ExecuteSubject(subject, a, instance); err != nil {
	// 	warning = append(warning, err.Error())
	// }
	// b, _ := ioutil.ReadAll(body)
	// sub, _ := ioutil.ReadAll(subject)
	// return struct {
	// 	Body    string
	// 	Subject string
	// 	Result  sched.States
	// 	Warning []string
	// }{
	// 	string(b),
	// 	string(sub),
	// 	res,
	// 	warning,
	// }, nil
}
