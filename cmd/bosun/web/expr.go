package web

import (
	"encoding/json"
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

	"bosun.org/cmd/bosun/cache"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/conf/rule"
	"bosun.org/cmd/bosun/expr"
	"bosun.org/cmd/bosun/sched"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/bradfitz/slice"
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
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// last line is expression we care about
		if i == len(lines)-1 {
			expression = schedule.RuleConf.Expand(line, vars, false)
		} else { // must be a variable declatation
			matches := varRegex.FindStringSubmatch(line)
			if len(matches) == 0 {
				return nil, fmt.Errorf("Expect all lines before final expression to be variable declarations of form `$foo = something`")
			}
			name := strings.TrimSpace(matches[1])
			value := strings.TrimSpace(matches[2])
			vars[name] = schedule.RuleConf.Expand(value, vars, false)
		}
	}
	e, err := expr.New(expression, schedule.RuleConf.GetFuncs(schedule.SystemConf.EnabledBackends()))
	if err != nil {
		return nil, err
	}
	now, err := getTime(r)
	if err != nil {
		return nil, err
	}
	// it may not strictly be necessary to recreate the contexts each time, but we do to be safe
	backends := &expr.Backends{
		TSDBContext:     schedule.SystemConf.GetTSDBContext(),
		GraphiteContext: schedule.SystemConf.GetGraphiteContext(),
		InfluxConfig:    schedule.SystemConf.GetInfluxContext(),
		ElasticHosts:    schedule.SystemConf.GetElasticContext(),
	}
	providers := &expr.BosunProviders{
		Cache:     cacheObj,
		Search:    schedule.Search,
		Squelched: nil,
		History:   nil,
		Annotate:  AnnotateBackend,
	}
	res, queries, err := e.Execute(backends, providers, t, now, 0, false)
	if err != nil {
		return nil, err
	}
	for _, r := range res.Results {
		if r.Computations == nil {
			r.Computations = make(models.Computations, 0)
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
			fd += " " + now.Format("15:04:05")
		}
		now, err = time.Parse("2006-01-02 15:04:05", fd)
		if err != nil {
			now, err = time.Parse("2006-01-02 15:04", fd)
		}
	}
	return
}

type Res struct {
	*models.Event
	Key models.AlertKey
}

func procRule(t miniprofiler.Timer, ruleConf conf.RuleConfProvider, a *conf.Alert, now time.Time, summary bool, email string, template_group string, incidentID int) (*ruleResult, error) {
	s := &sched.Schedule{}
	s.Search = schedule.Search
	if err := s.Init(schedule.SystemConf, ruleConf, schedule.DataAccess, AnnotateBackend, false, false); err != nil {
		return nil, err
	}
	rh := s.NewRunHistory(now, cacheObj)
	if _, err, _ := s.CheckExpr(t, rh, a, a.Warn, models.StWarning, nil); err != nil {
		return nil, err
	}
	if _, err, _ := s.CheckExpr(t, rh, a, a.Crit, models.StCritical, nil); err != nil {
		return nil, err
	}
	keys := make(models.AlertKeys, len(rh.Events))
	criticals, warnings, normals := make([]models.AlertKey, 0), make([]models.AlertKey, 0), make([]models.AlertKey, 0)
	i := 0
	for k, v := range rh.Events {
		v.Time = now
		keys[i] = k
		i++
		switch v.Status {
		case models.StNormal:
			normals = append(normals, k)
		case models.StWarning:
			warnings = append(warnings, k)
		case models.StCritical:
			criticals = append(criticals, k)
		default:
			return nil, fmt.Errorf("unknown state type %v", v.Status)
		}
	}
	sort.Sort(keys)
	var rt *models.RenderedTemplates
	var data interface{}
	var nots map[string]*conf.PreparedNotifications
	var aNots map[string]map[string]*conf.PreparedNotifications
	warning := make([]string, 0)

	if !summary && len(keys) > 0 {
		var primaryIncident *models.IncidentState
		if template_group != "" {
			ts, err := opentsdb.ParseTags(template_group)
			if err != nil {
				return nil, err
			}
			for _, ak := range keys {
				if ak.Group().Subset(ts) {
					primaryIncident = sched.NewIncident(ak)
					primaryIncident.Events = []models.Event{*rh.Events[ak]}
					break
				}
			}
		}
		if primaryIncident == nil {
			primaryIncident = sched.NewIncident(keys[0])
			primaryIncident.Events = []models.Event{*rh.Events[keys[0]]}
			if template_group != "" {
				warning = append(warning, fmt.Sprintf("template group %s was not a subset of any result", template_group))
			}
		}
		e := primaryIncident.Events[0]
		if e.Crit != nil {
			primaryIncident.Result = e.Crit
		} else if e.Warn != nil {
			primaryIncident.Result = e.Warn
		}
		var errs []error
		primaryIncident.Id = int64(incidentID)
		primaryIncident.Start = time.Now().UTC()
		primaryIncident.CurrentStatus = e.Status
		primaryIncident.LastAbnormalStatus = e.Status
		primaryIncident.LastAbnormalTime = models.Epoch{Time: time.Now().UTC()}
		func() {
			defer func() {
				if err := recover(); err != nil {
					s := fmt.Sprint(err)
					warning = append(warning, s)
					errs = append(errs, fmt.Errorf("panic rendering templates: %s", err))
				}
			}()
			rt, errs = s.ExecuteAll(rh, a, primaryIncident, false)
		}()
		for _, err := range errs {
			warning = append(warning, err.Error())
		}
		if rt == nil {
			rt = &models.RenderedTemplates{}
		}

		if len(errs) > 0 {
			var err error
			rt.Subject, rt.Body, err = s.ExecuteBadTemplate(errs, rh, a, primaryIncident)
			if err != nil {
				rt.Subject = fmt.Sprintf("unable to create tempalate error notification: %v", err)
			}
		} else if email != "" {
			m, err := mail.ParseAddress(email)
			if err != nil {
				return nil, err
			}
			n := conf.Notification{
				Email: []*mail.Address{m},
			}
			n.PrepareAlert(rt, string(primaryIncident.AlertKey), rt.Attachments...).Send(s.SystemConf)
		}
		nots, aNots = buildNotificationPreviews(a, rt, primaryIncident, s.SystemConf)
		data = s.Data(rh, primaryIncident, a, false)
	}

	rr := &ruleResult{
		Criticals:           criticals,
		Warnings:            warnings,
		Normals:             normals,
		Time:                now,
		Data:                data,
		Result:              rh.Events,
		Warning:             warning,
		RenderedTemplates:   rt,
		Notifications:       nots,
		ActionNotifications: aNots,
	}
	return rr, nil
}

func buildNotificationPreviews(a *conf.Alert, rt *models.RenderedTemplates, incident *models.IncidentState, c conf.SystemConfProvider, attachments ...*models.Attachment) (map[string]*conf.PreparedNotifications, map[string]map[string]*conf.PreparedNotifications) {
	previews := map[string]*conf.PreparedNotifications{}
	actionPreviews := map[string]map[string]*conf.PreparedNotifications{}
	nots := map[string]*conf.Notification{}
	for name, not := range a.CritNotification.GetAllChained() {
		nots[name] = not
	}
	for name, not := range a.WarnNotification.GetAllChained() {
		nots[name] = not
	}

	for name, not := range nots {
		previews[name] = not.PrepareAlert(rt, string(incident.AlertKey), attachments...)
		actions := map[string]*conf.PreparedNotifications{}
		actionPreviews[name] = actions
		// for all action types. just loop through known range. Update this if any get added
		for at := models.ActionAcknowledge; at <= models.ActionCancelClose; at++ {
			if !not.RunOnActionType(at) {
				continue
			}
			incidents := []*models.IncidentState{incident}
			actions[at.String()] = not.PrepareAction(at, a.Template, c, incidents, "somebody", "I took care of this")
		}
	}
	return previews, actionPreviews
}

type ruleResult struct {
	Criticals []models.AlertKey
	Warnings  []models.AlertKey
	Normals   []models.AlertKey
	Time      time.Time
	*models.RenderedTemplates
	Notifications       map[string]*conf.PreparedNotifications
	ActionNotifications map[string]map[string]*conf.PreparedNotifications
	Data                interface{}
	Result              map[models.AlertKey]*models.Event
	Warning             []string
}

func TestHTTPNotification(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	prep := &conf.PreparedHttp{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(prep); err != nil {
		return nil, err
	}
	code, err := prep.Send()
	dat := &struct {
		Error  string
		Status int
	}{"", code}
	if err != nil {
		dat.Error = err.Error()
	}
	return dat, nil
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
	incidentID := 42
	if incident := r.FormValue("incidentId"); len(incident) > 0 {
		incidentID, err = strconv.Atoi(incident)
		if err != nil {
			return nil, err
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
				res, err := procRule(t, c, a, now, interval != 0, r.FormValue("email"), r.FormValue("template_group"), incidentID)
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
		Group  models.AlertKey
		Result *models.Event
	}
	type Set struct {
		Critical, Warning, Normal int
		Time                      string
		Results                   []*Result `json:",omitempty"`
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
		AlertHistory map[models.AlertKey]*Histories
		*models.RenderedTemplates
		Notifications       map[string]*conf.PreparedNotifications
		ActionNotifications map[string]map[string]*conf.PreparedNotifications
		Data                interface{} `json:",omitempty"`
		Hash                string
	}{
		AlertHistory: make(map[models.AlertKey]*Histories),
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
			Critical: len(res.Criticals),
			Warning:  len(res.Warnings),
			Normal:   len(res.Normals),
			Time:     res.Time.Format(tsdbFormatSecs),
		}
		if res.Data != nil {
			ret.RenderedTemplates = res.RenderedTemplates
			ret.Notifications = res.Notifications
			ret.ActionNotifications = res.ActionNotifications
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

func buildConfig(r *http.Request) (c conf.RuleConfProvider, a *conf.Alert, hash string, err error) {
	config, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, nil, "", err
	}
	c, err = rule.NewConf("Test Config", schedule.SystemConf.EnabledBackends(), schedule.SystemConf.GetRuleVars(), string(config))
	if err != nil {
		return nil, nil, "", err
	}
	hash, err = sched.DefaultSched.DataAccess.Configs().SaveTempConfig(string(config))
	if err != nil {
		return nil, nil, "", err
	}
	alertName := r.FormValue("alert")
	if alertName == "" {
		return nil, nil, "", fmt.Errorf("must supply alert to run")
	}
	a = c.GetAlert(alertName)
	if a == nil {
		return nil, nil, "", fmt.Errorf("alert %s not found", alertName)
	}
	return c, a, hash, nil

}
