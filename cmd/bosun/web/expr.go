package web

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"bosun.org/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"bosun.org/_third_party/github.com/bradfitz/slice"
	"bosun.org/cmd/bosun/cache"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/expr"
	"bosun.org/cmd/bosun/sched"
	"bosun.org/opentsdb"
)

// for executing expressions/rules via the web UI, we use a cache that we retain during the lifetime of bosun
// Matt and I decided not to expire the cache at given points (such as reloading rule page), but I forgot why. ?
// the only risk is that if you query your store for data -5m to now and your store doesn't have the latest points up to date,
// and then 5m from now you query -10min to -5m you'll get the same cached data, including the incomplete last points
var cacheObj = cache.New(100)

func Expr(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (v interface{}, err error) {
	defer func() {
		if pan := recover(); pan != nil {
			v = nil
			err = fmt.Errorf("%v", pan)
		}
	}()
	text, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(text)), "\n")
	var expression string
	vars := map[string]string{}
	varRegex := regexp.MustCompile(`(\$\w+)\s*=(.*)`)
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// last line is expression we care about
		if i == len(lines)-1 {
			expression = schedule.Conf.Expand(line, vars, false)
		} else { // must be a variable declatation
			matches := varRegex.FindStringSubmatch(line)
			if len(matches) == 0 {
				return nil, fmt.Errorf("Expext all lines before final expression to be variable declarations of form `$foo = something`")
			}
			name := strings.TrimSpace(matches[1])
			value := strings.TrimSpace(matches[2])
			vars[name] = schedule.Conf.Expand(value, vars, false)
		}
	}
	e, err := expr.New(expression, schedule.Conf.Funcs())
	if err != nil {
		return nil, err
	}
	now, err := getTime(r)
	if err != nil {
		return nil, err
	}
	// it may not strictly be necessary to recreate the contexts each time, but we do to be safe
	tsdbContext := schedule.Conf.TSDBContext()
	graphiteContext := schedule.Conf.GraphiteContext()
	ls := schedule.Conf.LogstashElasticHosts
	res, queries, err := e.Execute(tsdbContext, graphiteContext, ls, cacheObj, t, now, 0, false, schedule.Search, nil, nil, schedule)
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

func procRule(t miniprofiler.Timer, c *conf.Conf, a *conf.Alert, now time.Time, summary bool, email string, template_group string) (*ruleResult, error) {
	s := &sched.Schedule{}
	if err := s.Init(c); err != nil {
		return nil, err
	}
	s.Metadata = schedule.Metadata
	s.Search = schedule.Search
	rh := s.NewRunHistory(now, cacheObj)
	if _, err := s.CheckExpr(t, rh, a, a.Warn, sched.StWarning, nil); err != nil {
		return nil, err
	}
	if _, err := s.CheckExpr(t, rh, a, a.Crit, sched.StCritical, nil); err != nil {
		return nil, err
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
			return nil, fmt.Errorf("unknown state type %v", v.Status)
		}
	}
	sort.Sort(keys)
	var subject, body []byte
	var data interface{}
	warning := make([]string, 0)
	if !summary && len(keys) > 0 {
		var instance *sched.State
		if template_group != "" {
			ts, err := opentsdb.ParseTags(template_group)
			if err != nil {
				return nil, err
			}
			for _, ak := range keys {
				if ak.Group().Subset(ts) {
					instance = s.GetOrCreateStatus(ak)
					instance.History = []sched.Event{*rh.Events[ak]}
					break
				}
			}
		}
		if instance == nil {
			instance = s.GetOrCreateStatus(keys[0])
			instance.History = []sched.Event{*rh.Events[keys[0]]}
			if template_group != "" {
				warning = append(warning, fmt.Sprintf("template group %s was not a subset of any result", template_group))
			}
		}
		if e := instance.History[0]; e.Error != nil {
			instance.Result = e.Error
		} else if e.Crit != nil {
			instance.Result = e.Crit
		} else if e.Warn != nil {
			instance.Result = e.Warn
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
			if body, _, b_err = s.ExecuteBody(rh, a, instance, false); b_err != nil {
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
			subject, s_err = s.ExecuteSubject(rh, a, instance, false)
			if s_err != nil {
				warning = append(warning, s_err.Error())
			}
		}()
		if s_err != nil || b_err != nil {
			var err error
			subject, body, err = s.ExecuteBadTemplate(s_err, b_err, rh, a, instance)
			if err != nil {
				subject = []byte(fmt.Sprintf("unable to create tempalate error notification: %v", err))
			}
		} else if email != "" {
			m, err := mail.ParseAddress(email)
			if err != nil {
				return nil, err
			}
			n := conf.Notification{
				Email: []*mail.Address{m},
			}
			email, attachments, b_err := s.ExecuteBody(rh, a, instance, true)
			email_subject, s_err := s.ExecuteSubject(rh, a, instance, true)
			if b_err != nil {
				warning = append(warning, b_err.Error())
			} else if s_err != nil {
				warning = append(warning, s_err.Error())
			} else {
				n.DoEmail(email_subject, email, schedule.Conf, string(instance.AlertKey()), attachments...)
			}
		}
		data = s.Data(rh, instance, a, false)
	}
	return &ruleResult{
		errors,
		criticals,
		warnings,
		normals,
		now,
		string(body),
		string(subject),
		data,
		rh.Events,
		warning,
	}, nil
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

	c, a, hash, err := buildConfig(r)
	if err != nil {
		return nil, err
	}

	ch := make(chan int)
	errch := make(chan error, intervals)
	resch := make(chan *ruleResult, intervals)
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
				res, err := procRule(t, c, a, now, interval != 0, r.FormValue("email"), r.FormValue("template_group"))
				resch <- res
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
		Hash         string
	}{
		AlertHistory: make(map[expr.AlertKey]*Histories),
		Hash:         hash,
	}
	for err := range errch {
		if err == nil {
			continue
		}
		ret.Errors = append(ret.Errors, err.Error())
	}
	for res := range resch {
		if res == nil {
			continue
		}
		set := Set{
			Error:    len(res.Errors),
			Critical: len(res.Criticals),
			Warning:  len(res.Warnings),
			Normal:   len(res.Normals),
			Time:     res.Time.Format(tsdbFormatSecs),
		}
		if res.Data != nil {
			ret.Body = res.Body
			ret.Subject = res.Subject
			ret.Data = res.Data
			for k, v := range res.Result {
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
		for k, v := range res.Result {
			if ret.AlertHistory[k] == nil {
				ret.AlertHistory[k] = new(Histories)
			}
			h := ret.AlertHistory[k]
			h.History = append(h.History, &History{
				Time:   v.Time,
				Status: v.Status.String(),
			})
		}
		ret.Sets = append(ret.Sets, &set)
		ret.Warnings = append(ret.Warnings, res.Warning...)
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

func buildConfig(r *http.Request) (c *conf.Conf, a *conf.Alert, hash string, err error) {
	config, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, nil, "", err
	}

	c, err = conf.New("Test Config", string(config))
	if err != nil {
		return nil, nil, "", err
	}
	c.StateFile = ""

	hash, err = sched.DefaultSched.SaveTempConfig(string(config))
	if err != nil {
		return nil, nil, "", err
	}

	alertName := r.FormValue("alert")
	if alertName == "" {
		return nil, nil, "", fmt.Errorf("must supply alert to run")
	}
	a, ok := c.Alerts[alertName]
	if !ok {
		return nil, nil, "", fmt.Errorf("alert %s not found", alertName)
	}
	return c, a, hash, nil

}
