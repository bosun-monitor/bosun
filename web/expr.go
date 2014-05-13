package web

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
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
	res, queries, err := e.Execute(opentsdb.NewCache(schedule.Conf.TsdbHost), t)
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
	all, queries, err := a.Crit.ExecuteOpts(opentsdb.NewCache(schedule.Conf.TsdbHost), t, now, 0)
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

func Template(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	// For now just hardcode the alert and template, will be taken from Form
	template := `template _t {
	body = <h1>Name: {{replace .Alert.Name "." " " -1}}</h1>
	subject = {{.Last.Status}}: {{replace .Alert.Name "." " " -1}}: {{.E .Alert.Vars.q}} on {{.Group.host}}
	}`
	txt := fmt.Sprintf(`
		tsdbHost = localhost:4242
		%s
		alert _ {
			template = _t
			$t = "5m"
			$q = avg(q("avg:rate{counter,,1}:os.cpu{host=*}", $t, ""))
			crit = $q > 0
		}`, template)
	c, err := conf.New("-", txt)
	if err != nil {
		return nil, err
	}
	a := c.Alerts["_"]
	s := &sched.Schedule{}
	s.Load(c)
	m := s.CheckExpr(a, a.Crit, 0, nil)
	var instance *sched.State
	// Just pick the first instance for now
	if len(m) > 1 {
		//Do something
	}
	for _, v := range m {
		instance = s.Status(v)
		break
	}
	body := new(bytes.Buffer)
	subject := new(bytes.Buffer)
	s.ExecuteBody(body, a, instance)
	s.ExecuteSubject(subject, a, instance)
	b, _ := ioutil.ReadAll(body)
	sub, _ := ioutil.ReadAll(subject)
	return struct {
		Body    string
		Subject string
	}{
		string(b),
		string(sub),
	}, nil
}
