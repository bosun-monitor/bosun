package sched // import "bosun.org/cmd/bosun/sched"

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	"bosun.org/annotate/backend"
	"bosun.org/cmd/bosun/cache"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/database"
	"bosun.org/cmd/bosun/search"
	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/bradfitz/slice"
	"github.com/kylebrandt/boolq"
)

// DefaultClient is the default http client for requests made from templates. It is configured in cmd/bosun/main.go
var DefaultClient *http.Client

func utcNow() time.Time {
	return time.Now().UTC()
}

type Schedule struct {
	mutex         sync.Mutex
	mutexHolder   string
	mutexAquired  time.Time
	mutexWaitTime int64

	RuleConf   conf.RuleConfProvider
	SystemConf conf.SystemConfProvider

	Search *search.Search

	annotate backend.Backend

	skipLast bool
	quiet    bool

	//channel signals an alert has added notifications, and notifications should be processed.
	nc chan interface{}
	//notifications to be sent immediately
	pendingNotifications map[*conf.Notification][]*IncidentWithTemplates

	//unknown states that need to be notified about. Collected and sent in batches.
	pendingUnknowns map[notificationGroupKey][]*models.IncidentState

	lastLogTimes map[models.AlertKey]time.Time
	LastCheck    time.Time

	ctx *checkContext

	DataAccess database.DataAccess

	// runnerContext is a context to track running alert routines
	runnerContext context.Context
	// cancelChecks is the function to call to cancel all alert routines
	cancelChecks context.CancelFunc
	// checksRunning waits for alert checks to finish before reloading
	// things that take significant time should be cancelled (i.e. expression execution)
	// whereas the runHistory is allowed to complete
	checksRunning sync.WaitGroup
}

func (s *Schedule) Init(systemConf conf.SystemConfProvider, ruleConf conf.RuleConfProvider, dataAccess database.DataAccess, annotate backend.Backend, skipLast, quiet bool) error {
	//initialize all variables and collections so they are ready to use.
	//this will be called once at app start, and also every time the rule
	//page runs, so be careful not to spawn long running processes that can't
	//be avoided.
	//var err error
	s.skipLast = skipLast
	s.quiet = quiet
	s.SystemConf = systemConf
	s.RuleConf = ruleConf
	s.annotate = annotate
	s.pendingUnknowns = make(map[notificationGroupKey][]*models.IncidentState)
	s.lastLogTimes = make(map[models.AlertKey]time.Time)
	s.LastCheck = utcNow()
	s.ctx = &checkContext{utcNow(), cache.New(0)}
	s.DataAccess = dataAccess
	// Initialize the context and waitgroup used to gracefully shutdown bosun as well as reload
	s.runnerContext, s.cancelChecks = context.WithCancel(context.Background())
	s.checksRunning = sync.WaitGroup{}

	if s.Search == nil {
		s.Search = search.NewSearch(s.DataAccess, skipLast)
	}
	return nil
}

type checkContext struct {
	runTime    time.Time
	checkCache *cache.Cache
}

func init() {
	metadata.AddMetricMeta(
		"bosun.schedule.lock_time", metadata.Counter, metadata.MilliSecond,
		"Length of time spent waiting for or holding the schedule lock.")
	metadata.AddMetricMeta(
		"bosun.schedule.lock_count", metadata.Counter, metadata.Count,
		"Number of times the given caller acquired the lock.")
}

func (s *Schedule) Lock(method string) {
	start := utcNow()
	s.mutex.Lock()
	s.mutexAquired = utcNow()
	s.mutexHolder = method
	s.mutexWaitTime = int64(s.mutexAquired.Sub(start) / time.Millisecond) // remember this so we don't have to call put until we leave the critical section.
}

func (s *Schedule) Unlock() {
	holder := s.mutexHolder
	start := s.mutexAquired
	waitTime := s.mutexWaitTime
	s.mutexHolder = ""
	s.mutex.Unlock()
	collect.Add("schedule.lock_time", opentsdb.TagSet{"caller": holder, "op": "wait"}, waitTime)
	collect.Add("schedule.lock_time", opentsdb.TagSet{"caller": holder, "op": "hold"}, int64(time.Since(start)/time.Millisecond))
	collect.Add("schedule.lock_count", opentsdb.TagSet{"caller": holder}, 1)
}

func (s *Schedule) GetLockStatus() (holder string, since time.Time) {
	return s.mutexHolder, s.mutexAquired
}

func (s *Schedule) PutMetadata(k metadata.Metakey, v interface{}) error {

	isCoreMeta := (k.Name == "desc" || k.Name == "unit" || k.Name == "rate")
	if !isCoreMeta {
		s.DataAccess.Metadata().PutTagMetadata(k.TagSet(), k.Name, fmt.Sprint(v), utcNow())
		return nil
	}
	if k.Metric == "" {
		err := fmt.Errorf("desc, rate, and unit require metric name")
		slog.Error(err)
		return err
	}
	return s.DataAccess.Metadata().PutMetricMetadata(k.Metric, k.Name, fmt.Sprint(v))
}

func (s *Schedule) DeleteMetadata(tags opentsdb.TagSet, name string) error {
	return s.DataAccess.Metadata().DeleteTagMetadata(tags, name)
}

func (s *Schedule) MetadataMetrics(metric string) (*database.MetricMetadata, error) {
	//denormalized metrics should give metric metadata for their undenormalized counterparts
	if strings.HasPrefix(metric, "__") {
		if idx := strings.Index(metric, "."); idx != -1 {
			metric = metric[idx+1:]
		}
	}
	mm, err := s.DataAccess.Metadata().GetMetricMetadata(metric)
	if err != nil {
		return nil, err
	}
	return mm, nil
}

func (s *Schedule) GetMetadata(metric string, subset opentsdb.TagSet) ([]metadata.Metasend, error) {
	ms := make([]metadata.Metasend, 0)
	if metric != "" {
		meta, err := s.MetadataMetrics(metric)
		if err != nil {
			return nil, err
		}
		if meta == nil {
			return nil, fmt.Errorf("metadata for metric %v not found", metric)
		}
		if meta.Desc != "" {
			ms = append(ms, metadata.Metasend{
				Metric: metric,
				Name:   "desc",
				Value:  meta.Desc,
			})
		}
		if meta.Unit != "" {
			ms = append(ms, metadata.Metasend{
				Metric: metric,
				Name:   "unit",
				Value:  meta.Unit,
			})
		}
		if meta.Rate != "" {
			ms = append(ms, metadata.Metasend{
				Metric: metric,
				Name:   "rate",
				Value:  meta.Rate,
			})
		}
	} else {
		meta, err := s.DataAccess.Metadata().GetTagMetadata(subset, "")
		if err != nil {
			return nil, err
		}
		for _, m := range meta {
			tm := time.Unix(m.LastTouched, 0)
			ms = append(ms, metadata.Metasend{
				Tags:  m.Tags,
				Name:  m.Name,
				Value: m.Value,
				Time:  &tm,
			})
		}
	}
	return ms, nil
}

type States map[models.AlertKey]*models.IncidentState

type StateTuple struct {
	NeedAck       bool
	Active        bool
	Status        models.Status
	CurrentStatus models.Status
	Silenced      bool
}

// GroupStates groups by NeedAck, Active, Status, and Silenced.
func (states States) GroupStates(silenced SilenceTester) map[StateTuple]States {
	r := make(map[StateTuple]States)
	for ak, st := range states {
		sil := silenced(ak) != nil
		t := StateTuple{
			NeedAck:       st.NeedAck,
			Active:        st.IsActive(),
			Status:        st.LastAbnormalStatus,
			CurrentStatus: st.CurrentStatus,
			Silenced:      sil,
		}
		if _, present := r[t]; !present {
			r[t] = make(States)
		}
		r[t][ak] = st
	}
	return r
}

// GroupSets returns slices of TagSets, grouped by most common ancestor. Those
// with no shared ancestor are grouped by alert name.
func (states States) GroupSets(minGroup int) map[string]models.AlertKeys {
	type Pair struct {
		k, v string
	}
	groups := make(map[string]models.AlertKeys)
	seen := make(map[*models.IncidentState]bool)
	for {
		counts := make(map[Pair]int)
		for _, s := range states {
			if seen[s] {
				continue
			}
			for k, v := range s.AlertKey.Group() {
				counts[Pair{k, v}]++
			}
		}
		if len(counts) == 0 {
			break
		}
		max := 0
		var pair Pair
		for p, c := range counts {
			if c > max {
				max = c
				pair = p
			}
		}
		if max < minGroup || minGroup <= 0 {
			break
		}
		var group models.AlertKeys
		for _, s := range states {
			if seen[s] {
				continue
			}
			if s.AlertKey.Group()[pair.k] != pair.v {
				continue
			}
			seen[s] = true
			group = append(group, s.AlertKey)
		}
		if len(group) > 0 {
			groups[fmt.Sprintf("{%s=%s}", pair.k, pair.v)] = group
		}
	}
	// alerts
	groupedByAlert := map[string]models.AlertKeys{}
	for _, s := range states {
		if seen[s] {
			continue
		}
		groupedByAlert[s.Alert] = append(groupedByAlert[s.Alert], s.AlertKey)
	}
	for a, aks := range groupedByAlert {
		if len(aks) >= minGroup {
			group := models.AlertKeys{}
			for _, ak := range aks {
				group = append(group, ak)
			}
			groups[a] = group
		}
	}
	// ungrouped
	for _, s := range states {
		if seen[s] || len(groupedByAlert[s.Alert]) >= minGroup {
			continue
		}
		groups[string(s.AlertKey)] = models.AlertKeys{s.AlertKey}
	}
	return groups
}

func (s *Schedule) GetOpenStates() (States, error) {
	incidents, err := s.DataAccess.State().GetAllOpenIncidents()
	if err != nil {
		return nil, err
	}
	states := make(States, len(incidents))
	for _, inc := range incidents {
		states[inc.AlertKey] = inc
	}
	return states, nil
}

type StateGroup struct {
	Active        bool `json:",omitempty"`
	Status        models.Status
	CurrentStatus models.Status
	Silenced      bool
	IsError       bool                  `json:",omitempty"`
	Subject       string                `json:",omitempty"`
	Alert         string                `json:",omitempty"`
	AlertKey      models.AlertKey       `json:",omitempty"`
	Ago           string                `json:",omitempty"`
	State         *models.IncidentState `json:",omitempty"`
	Children      []*StateGroup         `json:",omitempty"`
}

type StateGroups struct {
	Groups struct {
		NeedAck      []*StateGroup `json:",omitempty"`
		Acknowledged []*StateGroup `json:",omitempty"`
	}
	TimeAndDate                   []int
	FailingAlerts, UnclosedErrors int
}

func (s *Schedule) MarshalGroups(T miniprofiler.Timer, filter string) (*StateGroups, error) {
	var silenced SilenceTester
	T.Step("Silenced", func(miniprofiler.Timer) {
		silenced = s.Silenced()
	})
	var groups map[StateTuple]States
	var err error
	status := make(States)
	t := StateGroups{
		TimeAndDate: s.SystemConf.GetTimeAndDate(),
	}
	t.FailingAlerts, t.UnclosedErrors = s.getErrorCounts()
	T.Step("Setup", func(miniprofiler.Timer) {
		status2, err2 := s.GetOpenStates()
		if err2 != nil {
			err = err2
			return
		}
		var parsedExpr *boolq.Tree
		parsedExpr, err2 = boolq.Parse(filter)
		if err2 != nil {
			err = err2
			return
		}
		for k, v := range status2 {
			a := s.RuleConf.GetAlert(k.Name())
			if a == nil {
				slog.Errorf("unknown alert %s. Force closing.", k.Name())
				if err2 = s.ActionByAlertKey("bosun", "closing because alert doesn't exist.", models.ActionForceClose, nil, k); err2 != nil {
					slog.Error(err2)
				}
				continue
			}
			is, err2 := MakeIncidentSummary(s.RuleConf, silenced, v)
			if err2 != nil {
				err = err2
				return
			}
			match := false
			match, err2 = boolq.AskParsedExpr(parsedExpr, is)
			if err2 != nil {
				err = err2
				return
			}
			if match {
				status[k] = v
			}
		}
	})
	if err != nil {
		return nil, err
	}
	T.Step("GroupStates", func(T miniprofiler.Timer) {
		groups = status.GroupStates(silenced)
	})
	T.Step("groups", func(T miniprofiler.Timer) {
		for tuple, states := range groups {
			var grouped []*StateGroup
			switch tuple.Status {
			case models.StWarning, models.StCritical, models.StUnknown:
				var sets map[string]models.AlertKeys
				T.Step(fmt.Sprintf("GroupSets (%d): %v", len(states), tuple), func(T miniprofiler.Timer) {
					sets = states.GroupSets(s.SystemConf.GetMinGroupSize())
				})
				for name, group := range sets {
					g := StateGroup{
						Active:        tuple.Active,
						Status:        tuple.Status,
						CurrentStatus: tuple.CurrentStatus,
						Silenced:      tuple.Silenced,
						Subject:       fmt.Sprintf("%s - %s", tuple.Status, name),
					}
					for _, ak := range group {
						st := status[ak]
						g.Children = append(g.Children, &StateGroup{
							Active:   tuple.Active,
							Status:   tuple.Status,
							Silenced: tuple.Silenced,
							AlertKey: ak,
							Alert:    ak.Name(),
							Subject:  string(st.Subject),
							Ago:      marshalTime(st.Last().Time),
							State:    st,
							IsError:  !s.AlertSuccessful(ak.Name()),
						})
					}
					if len(g.Children) == 1 && g.Children[0].Subject != "" {
						g.Subject = g.Children[0].Subject
					}
					grouped = append(grouped, &g)
				}
			default:
				continue
			}
			if tuple.NeedAck {
				t.Groups.NeedAck = append(t.Groups.NeedAck, grouped...)
			} else {
				t.Groups.Acknowledged = append(t.Groups.Acknowledged, grouped...)
			}
		}
	})
	T.Step("sort", func(T miniprofiler.Timer) {
		gsort := func(grp []*StateGroup) func(i, j int) bool {
			return func(i, j int) bool {
				a := grp[i]
				b := grp[j]
				if a.Active && !b.Active {
					return true
				} else if !a.Active && b.Active {
					return false
				}
				if a.Status != b.Status {
					return a.Status > b.Status
				}
				if a.AlertKey != b.AlertKey {
					return a.AlertKey < b.AlertKey
				}
				return a.Subject < b.Subject
			}
		}
		slice.Sort(t.Groups.NeedAck, gsort(t.Groups.NeedAck))
		slice.Sort(t.Groups.Acknowledged, gsort(t.Groups.Acknowledged))
	})
	return &t, nil
}

func marshalTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	b, _ := t.MarshalText()
	return string(b)
}

var DefaultSched = &Schedule{}

// Load loads a configuration into the default schedule.
func Load(systemConf conf.SystemConfProvider, ruleConf conf.RuleConfProvider, dataAccess database.DataAccess, annotate backend.Backend, skipLast, quiet bool) error {
	return DefaultSched.Init(systemConf, ruleConf, dataAccess, annotate, skipLast, quiet)
}

// Run runs the default schedule.
func Run() error {
	return DefaultSched.Run()
}

func Close(reload bool) {
	DefaultSched.Close(reload)
}

func (s *Schedule) Close(reload bool) {
	s.cancelChecks()
	s.checksRunning.Wait()
	if s.skipLast || reload {
		return
	}
	err := s.Search.BackupLast()
	if err != nil {
		slog.Error(err)
	}
}

func (s *Schedule) Reset() {
	DefaultSched = &Schedule{}
}

func Reset() {
	DefaultSched.Reset()
}

func init() {
	metadata.AddMetricMeta("bosun.statefile.size", metadata.Gauge, metadata.Bytes,
		"The total size of the Bosun state file.")
	metadata.AddMetricMeta("bosun.check.duration", metadata.Gauge, metadata.Second,
		"The number of seconds it took Bosun to check each alert rule.")
	metadata.AddMetricMeta("bosun.check.err", metadata.Gauge, metadata.Error,
		"The running count of the number of errors Bosun has received while trying to evaluate an alert expression.")

	metadata.AddMetricMeta("bosun.actions", metadata.Gauge, metadata.Count,
		"The running count of actions performed by individual users (Closed alert, Acknowledged alert, etc).")
}

func (s *Schedule) ActionByAlertKey(user, message string, t models.ActionType, at *time.Time, ak models.AlertKey) error {
	st, err := s.DataAccess.State().GetLatestIncident(ak)
	if err != nil {
		return err
	}
	if st == nil {
		return fmt.Errorf("no such alert key: %v", ak)
	}
	_, err = s.action(user, message, t, at, st)
	return err
}

func (s *Schedule) ActionByIncidentId(user, message string, t models.ActionType, at *time.Time, id int64) (models.AlertKey, error) {
	st, err := s.DataAccess.State().GetIncidentState(id)
	if err != nil {
		return "", err
	}
	if st == nil {
		return "", fmt.Errorf("no incident with id: %v", id)
	}
	return s.action(user, message, t, at, st)
}

func (s *Schedule) action(user, message string, t models.ActionType, at *time.Time, st *models.IncidentState) (models.AlertKey, error) {
	isUnknown := st.LastAbnormalStatus == models.StUnknown
	timestamp := utcNow()
	action := models.Action{
		Message: message,
		Time:    timestamp,
		Type:    t,
		User:    user,
	}

	switch t {
	case models.ActionAcknowledge:
		if !st.NeedAck {
			return "", fmt.Errorf("alert already acknowledged")
		}
		if !st.Open {
			return "", fmt.Errorf("cannot acknowledge closed alert")
		}
		st.NeedAck = false
		if err := s.DataAccess.Notifications().ClearNotifications(st.AlertKey); err != nil {
			return "", err
		}
	case models.ActionCancelClose:
		found := false
		for i, a := range st.Actions {
			// Find first delayed close that hasn't already been fulfilled or canceled
			if a.Type == models.ActionDelayedClose && !(a.Fullfilled || a.Cancelled) {
				found, st.Actions[i].Cancelled = true, true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("no delayed close for incident %v (%v) found to cancel", st.Id, st.AlertKey)
		}
	case models.ActionClose:
		// closing effectively acks the incident
		st.NeedAck = false
		if st.IsActive() { // Closing an active incident results in delayed close
			var dl time.Time
			if at != nil {
				dl = *at
			} else {
				duration, err := s.GetCheckFrequency(st.AlertKey.Name())
				if err != nil {
					return "", err
				}
				dl = timestamp.Add(duration * 2)
			}
			// See if there is already a pending delayed close, if there is update the time and return
			for i, a := range st.Actions {
				if a.Type == models.ActionDelayedClose && !(a.Fullfilled || a.Cancelled) {
					st.Actions[i].Deadline = &dl
					_, err := s.DataAccess.State().UpdateIncidentState(st)
					if err != nil {
						return "", err
					}
				}
			}
			action.Type = models.ActionDelayedClose
			action.Deadline = &dl
		} else {
			st.Open = false
			st.End = &timestamp
		}
		if err := s.DataAccess.Notifications().ClearNotifications(st.AlertKey); err != nil {
			return "", err
		}
	case models.ActionForceClose:
		st.Open = false
		st.End = &timestamp
		if err := s.DataAccess.Notifications().ClearNotifications(st.AlertKey); err != nil {
			return "", err
		}
	case models.ActionForget:
		if !isUnknown {
			return "", fmt.Errorf("can only forget unknowns")
		}
		if err := s.DataAccess.Notifications().ClearNotifications(st.AlertKey); err != nil {
			return "", err
		}
		fallthrough
	case models.ActionPurge:
		if err := s.DataAccess.Notifications().ClearNotifications(st.AlertKey); err != nil {
			return "", err
		}
		return st.AlertKey, s.DataAccess.State().Forget(st.AlertKey)
	case models.ActionNote:
		// pass
	default:
		return "", fmt.Errorf("unknown action type: %v", t)
	}

	st.Actions = append(st.Actions, action)
	_, err := s.DataAccess.State().UpdateIncidentState(st)
	if err != nil {
		return "", err
	}
	if err := collect.Add("actions", opentsdb.TagSet{"user": user, "alert": st.AlertKey.Name(), "type": t.String()}, 1); err != nil {
		slog.Errorln(err)
	}
	return st.AlertKey, nil
}

type IncidentStatus struct {
	IncidentID         int64
	Active             bool
	AlertKey           models.AlertKey
	Status             models.Status
	StatusTime         int64
	Subject            string
	Silenced           bool
	LastAbnormalStatus models.Status
	LastAbnormalTime   models.Epoch
	NeedsAck           bool
}

func (s *Schedule) AlertSuccessful(name string) bool {
	b, err := s.DataAccess.Errors().IsAlertFailing(name)
	if err != nil {
		slog.Error(err)
		b = true
	}
	return !b
}

func (s *Schedule) markAlertError(name string, e error) {
	d := s.DataAccess.Errors()
	if err := d.MarkAlertFailure(name, e.Error()); err != nil {
		slog.Error(err)
		return
	}

}

func (s *Schedule) markAlertSuccessful(name string) {
	if err := s.DataAccess.Errors().MarkAlertSuccess(name); err != nil {
		slog.Error(err)
	}
}

func (s *Schedule) ClearErrors(alert string) error {
	if alert == "all" {
		return s.DataAccess.Errors().ClearAll()
	}
	return s.DataAccess.Errors().ClearAlert(alert)
}

func (s *Schedule) getErrorCounts() (failing, total int) {
	var err error
	failing, total, err = s.DataAccess.Errors().GetFailingAlertCounts()
	if err != nil {
		slog.Error(err)
	}
	return
}

func (s *Schedule) GetQuiet() bool {
	return s.quiet
}

// GetCheckFrequency returns the duration between checks for the named alert. If the alert
// does not exist an error is returned.
func (s *Schedule) GetCheckFrequency(alertName string) (time.Duration, error) {
	alertDef := s.RuleConf.GetAlert(alertName)
	if alertDef == nil {
		return 0, fmt.Errorf("can not get check frequency for alert %v, no such alert defined", alertName)
	}
	runEvery := alertDef.RunEvery
	if runEvery == 0 {
		runEvery = s.SystemConf.GetDefaultRunEvery()
	}
	return time.Duration(time.Duration(runEvery) * s.SystemConf.GetCheckFrequency()), nil

}
