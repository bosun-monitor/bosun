package web

import (
	"bytes"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"bosun.org/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"bosun.org/_third_party/github.com/bradfitz/slice"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/expr"
	"bosun.org/cmd/bosun/sched"
	"bosun.org/opentsdb"
)

func Expr(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	e, err := expr.New(r.FormValue("q"), schedule.Conf.Funcs())
	if err != nil {
		return nil, err
	}
	now, err := getTime(r)
	if err != nil {
		return nil, err
	}
	res, queries, err := e.Execute(schedule.Conf.TSDBCacheContext(), schedule.Conf.GraphiteContext(), schedule.Conf.LogstashElasticHost, t, now, 0, false, schedule.Search, nil)
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

func procRule(t miniprofiler.Timer, c *conf.Conf, a *conf.Alert, now time.Time, summary bool, email string, template_group string) (*ruleResult, *expr.Result, error) {
	s := &sched.Schedule{}
	s.Init(c)
	s.Metadata = schedule.Metadata
	s.Search = schedule.Search
	rh := s.NewRunHistory(now)
	var res *expr.Result
	var err error
	if _, res, err = s.CheckExpr(t, rh, a, a.Warn, sched.StWarning, nil); err != nil {
		return nil, nil, err
	}
	if _, res, err = s.CheckExpr(t, rh, a, a.Crit, sched.StCritical, nil); err != nil {
		return nil, nil, err
	}
	keys := make(expr.AlertKeys, len(rh.Events))
	errors, criticals, warnings, normals := make([]expr.AlertKey, 0), make([]expr.AlertKey, 0), make([]expr.AlertKey, 0), make([]expr.AlertKey, 0)
	i := 0
	for k, v := range rh.Events {
		v.Time = now
		keys[i] = k
		i++
		switch v.Status {
		case sched.StNormal:
			normals = append(normals, k)
		case sched.StWarning:
			warnings = append(warnings, k)
		case sched.StCritical:
			criticals = append(criticals, k)
		case sched.StError:
			errors = append(errors, k)
		default:
			return nil, nil, fmt.Errorf("unknown state type %v", v.Status)
		}
	}
	sort.Sort(keys)
	body := new(bytes.Buffer)
	subject := new(bytes.Buffer)
	var data interface{}
	warning := make([]string, 0)
	if !summary && len(keys) > 0 {
		var instance *sched.State
		if template_group != "" {
			ts, err := opentsdb.ParseTags(template_group)
			if err != nil {
				return nil, nil, err
			}
			for _, ak := range keys {
				if ak.Group().Subset(ts) {
					instance = s.Status(ak)
					instance.History = []sched.Event{*rh.Events[ak]}
					break
				}
			}
		}
		if instance == nil {
			instance = s.Status(keys[0])
			instance.History = []sched.Event{*rh.Events[keys[0]]}
			if template_group != "" {
				warning = append(warning, fmt.Sprintf("template group %s was not a subset of any result", template_group))
			}
		}
		var b_err, s_err error
		func() {
			defer func() {
				if err := recover(); err != nil {
					s := fmt.Sprint(err)
					warning = append(warning, s)
					b_err = fmt.Errorf(s)
				}
			}()
			if _, b_err = s.ExecuteBody(body, rh, a, instance, false); b_err != nil {
				warning = append(warning, b_err.Error())
			}
		}()
		func() {
			defer func() {
				if err := recover(); err != nil {
					s := fmt.Sprint(err)
					warning = append(warning, s)
					s_err = fmt.Errorf(s)
				}
			}()
			if s_err = s.ExecuteSubject(subject, rh, a, instance); s_err != nil {
				warning = append(warning, s_err.Error())
			}
		}()
		if s_err != nil || b_err != nil {
			var err error
			subject, body, err = s.ExecuteBadTemplate(s_err, b_err, rh, a, instance)
			if err != nil {
				subject = bytes.NewBufferString(fmt.Sprintf("unable to create tempalate error notification: %v", err))
			}
		} else if email != "" {
			m, err := mail.ParseAddress(email)
			if err != nil {
				return nil, nil, err
			}
			n := conf.Notification{
				Email: []*mail.Address{m},
			}
			email := new(bytes.Buffer)
			attachments, err := s.ExecuteBody(email, rh, a, instance, true)
			if err != nil {
				warning = append(warning, err.Error())
			} else {
				n.DoEmail(subject.Bytes(), email.Bytes(), schedule.Conf, string(instance.AlertKey()), attachments...)
			}
		}
		data = s.Data(rh, instance, a, false)
	}
	f := ruleResult{
		errors,
		criticals,
		warnings,
		normals,
		now,
		body.String(),
		subject.String(),
		data,
		rh.Events,
		warning,
	}
	return &f, res, nil
}

type ruleResult struct {
	Errors    []expr.AlertKey
	Criticals []expr.AlertKey
	Warnings  []expr.AlertKey
	Normals   []expr.AlertKey
	Time      time.Time

	Body    string
	Subject string
	Data    interface{}
	Result  map[expr.AlertKey]*sched.Event
	Warning []string
}

func Rule(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var from, to time.Time
	var err error
	if f := r.FormValue("from"); len(f) > 0 {
		from, err = time.Parse(tsdbFormatSecs, f)
		if err != nil {
			return nil, err
		}
	}
	if f := r.FormValue("to"); len(f) > 0 {
		to, err = time.Parse(tsdbFormatSecs, f)
		if err != nil {
			return nil, err
		}
	}
	intervals := 1
	if i := r.FormValue("intervals"); len(i) > 0 {
		intervals, err = strconv.Atoi(r.FormValue("intervals"))
		if err != nil {
			return nil, err
		}
		if intervals < 1 {
			return nil, fmt.Errorf("must be > 0 intervals")
		}
	}
	if fz, tz := from.IsZero(), to.IsZero(); fz && tz {
		from = time.Now()
	} else if fz && !tz {
		return nil, fmt.Errorf("cannot specify to without from")
	} else if !fz && tz && intervals > 1 {
		return nil, fmt.Errorf("cannot specify intervals without from and to")
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "tsdbHost = %s\n", schedule.Conf.TSDBHost)
	fmt.Fprintf(&buf, "graphiteHost = %s\n", schedule.Conf.GraphiteHost)
	fmt.Fprintf(&buf, "logstashElasticHost = %s\n", schedule.Conf.LogstashElasticHost)
	fmt.Fprintf(&buf, "smtpHost = %s\n", schedule.Conf.SMTPHost)
	fmt.Fprintf(&buf, "emailFrom = %s\n", schedule.Conf.EmailFrom)
	fmt.Fprintf(&buf, "responseLimit = %d\n", schedule.Conf.ResponseLimit)
	fmt.Fprintf(&buf, "hostname = %s\n", schedule.Conf.Hostname)
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
	var a *conf.Alert
	// Set a to the first alert.
	for _, a = range c.Alerts {
	}
	ch := make(chan int)
	type res struct {
		ruleRes *ruleResult
		exprRes *expr.Result
	}

	errch := make(chan error, intervals)
	resch := make(chan res, intervals)
	var wg sync.WaitGroup
	diff := -from.Sub(to)
	if intervals > 1 {
		diff /= time.Duration(intervals - 1)
	}
	worker := func() {
		wg.Add(1)
		for interval := range ch {
			t.Step(fmt.Sprintf("interval %v", interval), func(t miniprofiler.Timer) {
				now := from.Add(diff * time.Duration(interval))
				ruleRes, exprRes, err := procRule(t, c, a, now, interval != 0, r.FormValue("email"), r.FormValue("template_group"))
				resch <- res{ruleRes, exprRes}
				errch <- err
			})
		}
		defer wg.Done()
	}
	for i := 0; i < 20; i++ {
		go worker()
	}
	for i := 0; i < intervals; i++ {
		ch <- i
	}
	close(ch)
	wg.Wait()
	close(errch)
	close(resch)
	type Result struct {
		Group  expr.AlertKey
		Result *sched.Event
	}
	type Set struct {
		Error, Critical, Warning, Normal int
		Time                             string
		Results                          []*Result `json:",omitempty"`
	}
	type History struct {
		Time, EndTime time.Time
		Status        string
		exprRes       *expr.Result
	}
	type Histories struct {
		History []*History
	}
	ret := struct {
		Errors       []string `json:",omitempty"`
		Warnings     []string `json:",omitempty"`
		Sets         []*Set
		AlertHistory map[expr.AlertKey]*Histories
		Body         string      `json:",omitempty"`
		Subject      string      `json:",omitempty"`
		Data         interface{} `json:",omitempty"`
	}{
		AlertHistory: make(map[expr.AlertKey]*Histories),
	}
	for err := range errch {
		if err == nil {
			continue
		}
		ret.Errors = append(ret.Errors, err.Error())
	}
	for res := range resch {
		if res.ruleRes == nil {
			continue
		}
		set := Set{
			Error:    len(res.ruleRes.Errors),
			Critical: len(res.ruleRes.Criticals),
			Warning:  len(res.ruleRes.Warnings),
			Normal:   len(res.ruleRes.Normals),
			Time:     res.ruleRes.Time.Format(tsdbFormatSecs),
		}
		if res.ruleRes.Data != nil {
			ret.Body = res.ruleRes.Body
			ret.Subject = res.ruleRes.Subject
			ret.Data = res.ruleRes.Data
			for k, v := range res.ruleRes.Result {
				set.Results = append(set.Results, &Result{
					Group:  k,
					Result: v,
				})
			}
			slice.Sort(set.Results, func(i, j int) bool {
				a := set.Results[i]
				b := set.Results[j]
				if a.Result.Status != b.Result.Status {
					return a.Result.Status > b.Result.Status
				}
				return a.Group < b.Group
			})
		}
		for _, comp := range res.exprRes.Computations {
			fmt.Println(comp.Text, comp.Value)
		}

		for k, v := range res.ruleRes.Result {
			if ret.AlertHistory[k] == nil {
				ret.AlertHistory[k] = new(Histories)
			}
			h := ret.AlertHistory[k]
			h.History = append(h.History, &History{
				Time:    v.Time,
				Status:  v.Status.String(),
				exprRes: res.exprRes,
			})
		}
		ret.Sets = append(ret.Sets, &set)
		ret.Warnings = append(ret.Warnings, res.ruleRes.Warning...)
	}
	slice.Sort(ret.Sets, func(i, j int) bool {
		return ret.Sets[i].Time < ret.Sets[j].Time
	})
	for _, histories := range ret.AlertHistory {
		hist := histories.History
		slice.Sort(hist, func(i, j int) bool {
			return hist[i].Time.Before(hist[j].Time)
		})
		for i := 1; i < len(hist); i++ {
			if i < len(hist)-1 && hist[i].Status == hist[i-1].Status {
				hist = append(hist[:i], hist[i+1:]...)
				i--
			}
		}
		for i, h := range hist[:len(hist)-1] {
			h.EndTime = hist[i+1].Time
		}
		histories.History = hist[:len(hist)-1]
	}
	return &ret, nil
}
