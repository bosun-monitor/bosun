package sched // import "bosun.org/cmd/bosun/sched"

import (
	"encoding/gob"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	"bosun.org/cmd/bosun/cache"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/database"
	"bosun.org/cmd/bosun/expr"
	"bosun.org/cmd/bosun/search"
	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/boltdb/bolt"
	"github.com/bradfitz/slice"
	"github.com/kylebrandt/boolq"
	"github.com/tatsushid/go-fastping"
)

func utcNow() time.Time {
	return time.Now().UTC()
}

func init() {
	gob.Register(expr.Number(0))
	gob.Register(expr.Scalar(0))
}

type Schedule struct {
	mutex         sync.Mutex
	mutexHolder   string
	mutexAquired  time.Time
	mutexWaitTime int64

	Conf  *conf.Conf
	Group map[time.Time]models.AlertKeys

	Search *search.Search

	//channel signals an alert has added notifications, and notifications should be processed.
	nc chan interface{}
	//notifications to be sent immediately
	pendingNotifications map[*conf.Notification][]*models.IncidentState

	//unknown states that need to be notified about. Collected and sent in batches.
	pendingUnknowns map[*conf.Notification][]*models.IncidentState

	db *bolt.DB

	lastLogTimes map[models.AlertKey]time.Time
	LastCheck    time.Time

	ctx *checkContext

	DataAccess database.DataAccess

	runnerContext context.Context
	cancelChecks  context.CancelFunc
	checksRunning sync.WaitGroup
}

func (s *Schedule) Init(c *conf.Conf) error {
	//initialize all variables and collections so they are ready to use.
	//this will be called once at app start, and also every time the rule
	//page runs, so be careful not to spawn long running processes that can't
	//be avoided.
	//var err error
	s.Conf = c
	s.Group = make(map[time.Time]models.AlertKeys)
	s.pendingUnknowns = make(map[*conf.Notification][]*models.IncidentState)
	s.lastLogTimes = make(map[models.AlertKey]time.Time)
	s.LastCheck = utcNow()
	s.ctx = &checkContext{utcNow(), cache.New(0)}

	s.runnerContext, s.cancelChecks = context.WithCancel(context.Background())
	s.checksRunning = sync.WaitGroup{}

	if s.DataAccess == nil {
		if c.RedisHost != "" {
			s.DataAccess = database.NewDataAccess(c.RedisHost, true, c.RedisDb, c.RedisPassword)
		} else {
			_, err := database.StartLedis(c.LedisDir, c.LedisBindAddr)
			if err != nil {
				return err
			}
			s.DataAccess = database.NewDataAccess(c.LedisBindAddr, false, 0, "")
		}
	}
	if s.Search == nil {
		s.Search = search.NewSearch(s.DataAccess, c.SkipLast)
	}
	// if c.StateFile != "" {
	// 	s.db, err = bolt.Open(c.StateFile, 0600, nil)
	// 	if err != nil {
	// 		return err
	// 	}
	// }
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
	strVal, ok := v.(string)
	if !ok {
		err := fmt.Errorf("desc, rate, and unit require value to be string. Found: %s", reflect.TypeOf(v))
		slog.Error(err)
		return err
	}
	return s.DataAccess.Metadata().PutMetricMetadata(k.Metric, k.Name, strVal)
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
		if max < minGroup {
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
		TimeAndDate: s.Conf.TimeAndDate,
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
			a := s.Conf.Alerts[k.Name()]
			if a == nil {
				slog.Errorf("unknown alert %s. Force closing.", k.Name())
				if err2 = s.ActionByAlertKey("bosun", "closing because alert doesn't exist.", models.ActionForceClose, k); err2 != nil {
					slog.Error(err2)
				}
				continue
			}
			is := MakeIncidentSummary(s.Conf, silenced, v)
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
					sets = states.GroupSets(s.Conf.MinGroupSize)
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
						st.Body = ""
						st.EmailBody = nil
						st.Attachments = nil
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
func Load(c *conf.Conf) error {
	return DefaultSched.Load(c)
}

// Run runs the default schedule.
func Run() error {
	return DefaultSched.Run()
}

func (s *Schedule) Load(c *conf.Conf) error {
	if err := s.Init(c); err != nil {
		return err
	}
	if s.db == nil {
		return nil
	}
	return s.RestoreState()
}

func Close(reload bool) {
	DefaultSched.Close(reload)
}

func (s *Schedule) Close(reload bool) {
	s.cancelChecks()
	s.checksRunning.Wait()
	if s.Conf.SkipLast || reload {
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

const pingFreq = time.Second * 15

func (s *Schedule) PingHosts() {
	for range time.Tick(pingFreq) {
		hosts, err := s.Search.TagValuesByTagKey("host", s.Conf.PingDuration)
		if err != nil {
			slog.Error(err)
			continue
		}
		for _, host := range hosts {
			go pingHost(host)
		}
	}
}

func pingHost(host string) {
	p := fastping.NewPinger()
	tags := opentsdb.TagSet{"dst_host": host}
	resolved := 0
	defer func() {
		collect.Put("ping.resolved", tags, resolved)
	}()
	ra, err := net.ResolveIPAddr("ip4:icmp", host)
	if err != nil {
		return
	}
	resolved = 1
	p.AddIPAddr(ra)
	p.MaxRTT = time.Second * 5
	timeout := 1
	p.OnRecv = func(addr *net.IPAddr, t time.Duration) {
		collect.Put("ping.rtt", tags, float64(t)/float64(time.Millisecond))
		timeout = 0
	}
	if err := p.Run(); err != nil {
		slog.Errorln(err)
	}
	collect.Put("ping.timeout", tags, timeout)
}

func init() {
	metadata.AddMetricMeta("bosun.statefile.size", metadata.Gauge, metadata.Bytes,
		"The total size of the Bosun state file.")
	metadata.AddMetricMeta("bosun.check.duration", metadata.Gauge, metadata.Second,
		"The number of seconds it took Bosun to check each alert rule.")
	metadata.AddMetricMeta("bosun.check.err", metadata.Gauge, metadata.Error,
		"The running count of the number of errors Bosun has received while trying to evaluate an alert expression.")
	metadata.AddMetricMeta("bosun.ping.resolved", metadata.Gauge, metadata.Bool,
		"1=Ping resolved to an IP Address. 0=Ping failed to resolve to an IP Address.")
	metadata.AddMetricMeta("bosun.ping.rtt", metadata.Gauge, metadata.MilliSecond,
		"The number of milliseconds for the echo reply to be received. Also known as Round Trip Time.")
	metadata.AddMetricMeta("bosun.ping.timeout", metadata.Gauge, metadata.Ok,
		"0=Ping responded before timeout. 1=Ping did not respond before 5 second timeout.")
	metadata.AddMetricMeta("bosun.actions", metadata.Gauge, metadata.Count,
		"The running count of actions performed by individual users (Closed alert, Acknowledged alert, etc).")
}

func (s *Schedule) ActionByAlertKey(user, message string, t models.ActionType, ak models.AlertKey) error {
	st, err := s.DataAccess.State().GetLatestIncident(ak)
	if err != nil {
		return err
	}
	if st == nil {
		return fmt.Errorf("no such alert key: %v", ak)
	}
	_, err = s.action(user, message, t, st)
	return err
}

func (s *Schedule) ActionByIncidentId(user, message string, t models.ActionType, id int64) (models.AlertKey, error) {
	st, err := s.DataAccess.State().GetIncidentState(id)
	if err != nil {
		return "", err
	}
	if st == nil {
		return "", fmt.Errorf("no incident with id: %v", id)
	}
	return s.action(user, message, t, st)
}

func (s *Schedule) action(user, message string, t models.ActionType, st *models.IncidentState) (ak models.AlertKey, e error) {
	if err := collect.Add("actions", opentsdb.TagSet{"user": user, "alert": st.AlertKey.Name(), "type": t.String()}, 1); err != nil {
		slog.Errorln(err)
	}
	defer func() {
		if e == nil {
			if err := collect.Add("actions", opentsdb.TagSet{"user": user, "alert": st.AlertKey.Name(), "type": t.String()}, 1); err != nil {
				slog.Errorln(err)
			}
			if err := s.DataAccess.Notifications().ClearNotifications(st.AlertKey); err != nil {
				e = err
			}
		}
	}()
	isUnknown := st.LastAbnormalStatus == models.StUnknown
	timestamp := utcNow()
	switch t {
	case models.ActionAcknowledge:
		if !st.NeedAck {
			return "", fmt.Errorf("alert already acknowledged")
		}
		if !st.Open {
			return "", fmt.Errorf("cannot acknowledge closed alert")
		}
		st.NeedAck = false
	case models.ActionClose:
		if st.IsActive() {
			return "", fmt.Errorf("cannot close active alert")
		}
		fallthrough
	case models.ActionForceClose:
		st.Open = false
		st.End = &timestamp
	case models.ActionForget:
		if !isUnknown {
			return "", fmt.Errorf("can only forget unknowns")
		}
		fallthrough
	case models.ActionPurge:
		return st.AlertKey, s.DataAccess.State().Forget(st.AlertKey)
	case models.ActionNote:
		// pass
	default:
		return "", fmt.Errorf("unknown action type: %v", t)
	}
	st.Actions = append(st.Actions, models.Action{
		Message: message,
		Time:    timestamp,
		Type:    t,
		User:    user,
	})
	_, err := s.DataAccess.State().UpdateIncidentState(st)
	return st.AlertKey, err
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
	LastAbnormalTime   int64
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
