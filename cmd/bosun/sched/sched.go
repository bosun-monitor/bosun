package sched // import "bosun.org/cmd/bosun/sched"

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sort"
	"sync"
	"time"

	"bosun.org/_third_party/github.com/boltdb/bolt"

	"bosun.org/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"bosun.org/_third_party/github.com/bradfitz/slice"
	"bosun.org/_third_party/github.com/tatsushid/go-fastping"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/expr"
	"bosun.org/cmd/bosun/search"
	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	gob.Register(expr.Number(0))
	gob.Register(expr.Scalar(0))
}

type Schedule struct {
	mutex         sync.Mutex
	mutexHolder   string
	mutexAquired  time.Time
	mutexWaitTime int64

	Conf          *conf.Conf
	status        States
	readStatus    States // READ-ONLY copy of status. Updated after each runHistory
	Notifications map[expr.AlertKey]map[string]time.Time
	Silence       map[string]*Silence
	Group         map[time.Time]expr.AlertKeys
	Metadata      map[metadata.Metakey]*Metavalue
	Incidents     map[uint64]*Incident
	Search        *search.Search

	LastCheck     time.Time
	nc            chan interface{}
	notifications map[*conf.Notification][]*State
	metalock      sync.Mutex
	maxIncidentId uint64
	incidentLock  sync.Mutex
	db            *bolt.DB
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
	start := time.Now()
	s.mutex.Lock()
	s.mutexAquired = time.Now()
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

type Metavalue struct {
	Time  time.Time
	Value interface{}
}

func (s *Schedule) PutMetadata(k metadata.Metakey, v interface{}) {
	s.metalock.Lock()
	s.Metadata[k] = &Metavalue{time.Now().UTC(), v}
	s.metalock.Unlock()
}

type MetadataMetric struct {
	Unit        string `json:",omitempty"`
	Type        string `json:",omitempty"`
	Description []*MetadataDescription
}

type MetadataDescription struct {
	Tags opentsdb.TagSet `json:",omitempty"`
	Text string
}

func (s *Schedule) MetadataMetrics(metric string) map[string]*MetadataMetric {
	s.metalock.Lock()
	m := make(map[string]*MetadataMetric)
	for k, mv := range s.Metadata {
		tags := k.TagSet()
		delete(tags, "host")
		if k.Metric == "" || (metric != "" && k.Metric != metric) {
			continue
		}
		val, _ := mv.Value.(string)
		if val == "" {
			continue
		}
		if m[k.Metric] == nil {
			m[k.Metric] = &MetadataMetric{
				Description: make([]*MetadataDescription, 0),
			}
		}
		e := m[k.Metric]
	Switch:
		switch k.Name {
		case "unit":
			e.Unit = val
		case "rate":
			e.Type = val
		case "desc":
			for _, v := range e.Description {
				if v.Text == val {
					v.Tags = v.Tags.Intersection(tags)
					break Switch
				}
			}
			e.Description = append(e.Description, &MetadataDescription{
				Text: val,
				Tags: tags,
			})
		}
	}
	s.metalock.Unlock()
	return m
}

func (s *Schedule) GetMetadata(metric string, subset opentsdb.TagSet) []metadata.Metasend {
	s.metalock.Lock()
	ms := make([]metadata.Metasend, 0)
	for k, mv := range s.Metadata {
		if metric != "" && k.Metric != metric {
			continue
		}
		if !k.TagSet().Subset(subset) {
			continue
		}
		ms = append(ms, metadata.Metasend{
			Metric: k.Metric,
			Tags:   k.TagSet(),
			Name:   k.Name,
			Value:  mv.Value,
			Time:   &mv.Time,
		})
	}
	s.metalock.Unlock()
	return ms
}

type States map[expr.AlertKey]*State

type StateTuple struct {
	NeedAck  bool
	Active   bool
	Status   Status
	Silenced bool
}

// GroupStates groups by NeedAck, Active, Status, and Silenced.
func (states States) GroupStates(silenced map[expr.AlertKey]Silence) map[StateTuple]States {
	r := make(map[StateTuple]States)
	for ak, st := range states {
		_, sil := silenced[ak]
		t := StateTuple{
			st.NeedAck,
			st.IsActive(),
			st.AbnormalStatus(),
			sil,
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
func (states States) GroupSets() map[string]expr.AlertKeys {
	type Pair struct {
		k, v string
	}
	groups := make(map[string]expr.AlertKeys)
	seen := make(map[*State]bool)
	for {
		counts := make(map[Pair]int)
		for _, s := range states {
			if seen[s] {
				continue
			}
			for k, v := range s.Group {
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
		if max == 1 {
			break
		}
		var group expr.AlertKeys
		for _, s := range states {
			if seen[s] {
				continue
			}
			if s.Group[pair.k] != pair.v {
				continue
			}
			seen[s] = true
			group = append(group, s.AlertKey())
		}
		if len(group) > 0 {
			groups[fmt.Sprintf("{%s=%s}", pair.k, pair.v)] = group
		}
	}
	// alerts
	for {
		if len(seen) == len(states) {
			break
		}
		var group expr.AlertKeys
		for _, s := range states {
			if seen[s] {
				continue
			}
			if group == nil || s.AlertKey().Name() == group[0].Name() {
				group = append(group, s.AlertKey())
				seen[s] = true
			}
		}
		if len(group) > 0 {
			groups[group[0].Name()] = group
		}
	}
	return groups
}

func (states States) Copy() States {
	newStates := make(States, len(states))
	for ak, st := range states {
		newStates[ak] = st.Copy()
	}
	return newStates
}

type StateGroup struct {
	Active   bool `json:",omitempty"`
	Status   Status
	Silenced bool
	Subject  string        `json:",omitempty"`
	Alert    string        `json:",omitempty"`
	AlertKey expr.AlertKey `json:",omitempty"`
	Ago      string        `json:",omitempty"`
	State    *State        `json:",omitempty"`
	Children []*StateGroup `json:",omitempty"`
}

type StateGroups struct {
	Groups struct {
		NeedAck      []*StateGroup `json:",omitempty"`
		Acknowledged []*StateGroup `json:",omitempty"`
	}
	TimeAndDate []int
}

func (s *Schedule) MarshalGroups(T miniprofiler.Timer, filter string) (*StateGroups, error) {
	var silenced map[expr.AlertKey]Silence
	T.Step("Silenced", func(miniprofiler.Timer) {
		silenced = s.Silenced()
	})
	var groups map[StateTuple]States
	var err error
	status := make(States)
	t := StateGroups{
		TimeAndDate: s.Conf.TimeAndDate,
	}
	T.Step("Setup", func(miniprofiler.Timer) {

		matches, err2 := makeFilter(filter)
		if err2 != nil {
			err = err2
			return
		}
		for k, v := range s.readStatus {
			if !v.Open {
				continue
			}
			a := s.Conf.Alerts[k.Name()]
			if a == nil {
				err = fmt.Errorf("unknown alert %s", k.Name())
				return
			}
			if matches(s.Conf, a, v) {
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
			case StWarning, StCritical, StUnknown, StError:
				var sets map[string]expr.AlertKeys
				T.Step(fmt.Sprintf("GroupSets (%d): %v", len(states), tuple), func(T miniprofiler.Timer) {
					sets = states.GroupSets()
				})
				for name, group := range sets {
					g := StateGroup{
						Active:   tuple.Active,
						Status:   tuple.Status,
						Silenced: tuple.Silenced,
						Subject:  fmt.Sprintf("%s - %s", tuple.Status, name),
					}
					for _, ak := range group {
						st := s.status[ak]
						g.Children = append(g.Children, &StateGroup{
							Active:   tuple.Active,
							Status:   tuple.Status,
							Silenced: tuple.Silenced,
							AlertKey: ak,
							Alert:    ak.Name(),
							Subject:  string(st.Subject),
							Ago:      marshalTime(st.Last().Time),
							State:    st,
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

func (s *Schedule) Init(c *conf.Conf) error {
	var err error
	s.Conf = c
	s.Silence = make(map[string]*Silence)
	s.Group = make(map[time.Time]expr.AlertKeys)
	s.Metadata = make(map[metadata.Metakey]*Metavalue)
	s.Incidents = make(map[uint64]*Incident)
	s.status = make(States)
	s.Search = search.NewSearch()
	if c.StateFile != "" {
		s.db, err = bolt.Open(c.StateFile, 0600, nil)
		if err != nil {
			return err
		}
	}
	s.readStatus = s.status.Copy()
	return nil
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

func Close() {
	DefaultSched.Close()
}

func (s *Schedule) Close() {
	s.save()
	s.Lock("Close")
	if s.db != nil {
		s.db.Close()
	}
	s.Unlock()
}

func (s *Schedule) Run() error {
	if s.Conf == nil {
		return fmt.Errorf("sched: nil configuration")
	}
	s.nc = make(chan interface{}, 1)
	if s.Conf.Ping {
		go s.PingHosts()
	}
	go s.Poll()
	go s.performSave()
	interval := uint64(0)
	for {
		wait := time.After(s.Conf.CheckFrequency)
		log.Println("starting check")
		now := time.Now()
		dur, err := s.Check(nil, now, interval)
		if err != nil {
			log.Println(err)
		}
		log.Printf("check took %v\n", dur)
		s.LastCheck = now
		<-wait
		interval++
	}
}

const pingFreq = time.Second * 15

func (s *Schedule) PingHosts() {
	for range time.Tick(pingFreq) {
		hosts := s.Search.TagValuesByTagKey("host", s.Conf.PingDuration)
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
		log.Print(err)
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

type State struct {
	*Result

	// Most recent last.
	History          []Event  `json:",omitempty"`
	Actions          []Action `json:",omitempty"`
	Touched          time.Time
	Alert            string // helper data since AlertKeys don't serialize to JSON well
	Tags             string // string representation of Group
	Group            opentsdb.TagSet
	Subject          string
	Body             string
	EmailBody        []byte             `json:"-"`
	EmailSubject     []byte             `json:"-"`
	Attachments      []*conf.Attachment `json:"-"`
	NeedAck          bool
	Open             bool
	Forgotten        bool
	Unevaluated      bool
	LastLogTime      time.Time
	TemplateWarnings []string
}

func (s *State) Copy() *State {
	newState := &State{
		History:          make([]Event, len(s.History)),
		Actions:          make([]Action, len(s.Actions)),
		Touched:          s.Touched,
		Alert:            s.Alert,
		Tags:             s.Tags,
		Group:            s.Group.Copy(),
		Subject:          s.Subject,
		Body:             s.Body,
		EmailBody:        s.EmailBody,
		EmailSubject:     s.EmailSubject,
		Attachments:      s.Attachments,
		NeedAck:          s.NeedAck,
		Open:             s.Open,
		Forgotten:        s.Forgotten,
		Unevaluated:      s.Unevaluated,
		LastLogTime:      s.LastLogTime,
		TemplateWarnings: s.TemplateWarnings,
	}
	newState.Result = s.Result
	for i := range s.History {
		newState.History[i] = s.History[i]
	}
	for i := range s.Actions {
		newState.Actions[i] = s.Actions[i]
	}
	return newState
}

func (s *State) AlertKey() expr.AlertKey {
	return expr.NewAlertKey(s.Alert, s.Group)
}

func (s *State) Status() Status {
	return s.Last().Status
}

// AbnormalEvent returns the most recent non-normal event, or nil if none found.
func (s *State) AbnormalEvent() *Event {
	for i := len(s.History) - 1; i >= 0; i-- {
		if ev := s.History[i]; ev.Status > StNormal {
			return &ev
		}
	}
	return nil
}

// AbnormalStatus returns the most recent non-normal status, or StNone if none
// found.
func (s *State) AbnormalStatus() Status {
	ev := s.AbnormalEvent()
	if ev != nil {
		return ev.Status
	}
	return StNone
}

func (s *State) IsActive() bool {
	return s.Status() > StNormal
}

func (s *State) Action(user, message string, t ActionType, timestamp time.Time) {
	s.Actions = append(s.Actions, Action{
		User:    user,
		Message: message,
		Type:    t,
		Time:    timestamp,
	})
}

func (s *Schedule) Action(user, message string, t ActionType, ak expr.AlertKey) error {
	s.Lock("Action")
	defer s.Unlock()
	st := s.status[ak]
	if st == nil {
		return fmt.Errorf("no such alert key: %v", ak)
	}
	ack := func() {
		delete(s.Notifications, ak)
		st.NeedAck = false
	}
	isUnknown := st.AbnormalStatus() == StUnknown
	isError := st.AbnormalStatus() == StError
	timestamp := time.Now().UTC()
	switch t {
	case ActionAcknowledge:
		if !st.NeedAck {
			return fmt.Errorf("alert already acknowledged")
		}
		if !st.Open {
			return fmt.Errorf("cannot acknowledge closed alert")
		}
		ack()
	case ActionClose:
		if st.NeedAck {
			ack()
		}
		if st.IsActive() && !isError {
			return fmt.Errorf("cannot close active alert")
		}
		st.Open = false
		last := st.Last()
		if last.IncidentId != 0 {
			s.incidentLock.Lock()
			if incident, ok := s.Incidents[last.IncidentId]; ok {
				incident.End = &timestamp
			}
			s.incidentLock.Unlock()
		}
	case ActionForget:
		if !isUnknown {
			return fmt.Errorf("can only forget unknowns")
		}
		if st.NeedAck {
			ack()
		}
		st.Open = false
		st.Forgotten = true
		delete(s.status, ak)
	default:
		return fmt.Errorf("unknown action type: %v", t)
	}
	st.Action(user, message, t, timestamp)
	// Would like to also track the alert group, but I believe this is impossible because any character
	// that could be used as a delimiter could also be a valid tag key or tag value character
	if err := collect.Add("actions", opentsdb.TagSet{"user": user, "alert": ak.Name(), "type": t.String()}, 1); err != nil {
		log.Println(err)
	}
	return nil
}

func (s *State) Touch() {
	s.Touched = time.Now().UTC()
	s.Forgotten = false
}

// Append appends status to the history if the status is different than the
// latest status. Returns the previous status.
func (s *State) Append(event *Event) Status {
	last := s.Last()
	if len(s.History) == 0 || last.Status != event.Status {
		s.History = append(s.History, *event)
	}
	return last.Status
}

func (s *State) Last() Event {
	if len(s.History) == 0 {
		return Event{}
	}
	return s.History[len(s.History)-1]
}

type Event struct {
	Warn, Crit, Error *Result
	Status            Status
	Time              time.Time
	Unevaluated       bool
	IncidentId        uint64
}

type Result struct {
	*expr.Result
	Expr string
}

func (r *Result) Copy() *Result {
	return &Result{r.Result, r.Expr}
}

type Status int

const (
	StNone Status = iota
	StNormal
	StWarning
	StCritical
	StUnknown
	StError
)

func (s Status) String() string {
	switch s {
	case StNormal:
		return "normal"
	case StWarning:
		return "warning"
	case StCritical:
		return "critical"
	case StUnknown:
		return "unknown"
	case StError:
		return "error"
	default:
		return "none"
	}
}

func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s Status) IsNormal() bool   { return s == StNormal }
func (s Status) IsWarning() bool  { return s == StWarning }
func (s Status) IsCritical() bool { return s == StCritical }
func (s Status) IsUnknown() bool  { return s == StUnknown }
func (s Status) IsError() bool    { return s == StError }

type Action struct {
	User    string
	Message string
	Time    time.Time
	Type    ActionType
}

type ActionType int

const (
	ActionNone ActionType = iota
	ActionAcknowledge
	ActionClose
	ActionForget
)

func (a ActionType) String() string {
	switch a {
	case ActionAcknowledge:
		return "Acknowledged"
	case ActionClose:
		return "Closed"
	case ActionForget:
		return "Forgotten"
	default:
		return "none"
	}
}

func (a ActionType) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

type Incident struct {
	Id       uint64
	Start    time.Time
	End      *time.Time
	AlertKey expr.AlertKey
}

func (s *Schedule) createIncident(ak expr.AlertKey, start time.Time) *Incident {
	s.incidentLock.Lock()
	defer s.incidentLock.Unlock()
	s.maxIncidentId++
	id := s.maxIncidentId
	incident := &Incident{
		Id:       id,
		Start:    start,
		AlertKey: ak,
	}

	s.Incidents[id] = incident
	return incident
}

type incidentList []*Incident

func (i incidentList) Len() int { return len(i) }
func (i incidentList) Less(a int, b int) bool {
	if i[a].Start.Before(i[b].Start) {
		return true
	}
	return i[a].AlertKey < i[b].AlertKey
}
func (i incidentList) Swap(a int, b int) { i[a], i[b] = i[b], i[a] }

func (s *Schedule) createHistoricIncidents() {
	incidents := make(incidentList, 0)
	indexes := make(map[*Incident]int)
	s.Incidents = make(map[uint64]*Incident)
	// 1. Create all incidents, but don't assign ids or link events yet.
	for ak, state := range s.status {
		var currentIncident *Incident
		for i, ev := range state.History {
			if currentIncident != nil {
				if currentIncident.End == nil || ev.Time.Before(*currentIncident.End) {
					// Continue open incident
					continue
				} else {
					// End incident after end time
					currentIncident = nil
				}
			}
			if ev.Status == StNormal {
				continue
			}
			// New incident
			currentIncident = &Incident{AlertKey: ak, Start: ev.Time}
			indexes[currentIncident] = i
			incidents = append(incidents, currentIncident)
			// Find end time for incident
			for _, action := range state.Actions {
				if action.Type == ActionClose && action.Time.After(ev.Time) {
					end := action.Time
					currentIncident.End = &end
					break
				}
			}
		}
	}
	// 2. Sort incidents
	sort.Sort(incidents)
	// 3. Assign ids and link events to appropriate ids
	for _, incident := range incidents {
		s.maxIncidentId++
		incident.Id = s.maxIncidentId
		// Find events and mark them
		state := s.status[incident.AlertKey]
		for idx := indexes[incident]; idx < len(state.History); idx++ {
			ev := state.History[idx]
			if incident.End == nil || ev.Time.Before(*incident.End) {
				ev.IncidentId = incident.Id
				state.History[idx] = ev
			} else {
				break
			}
		}
		s.Incidents[incident.Id] = incident
	}
}

func (s *Schedule) GetIncidents(alert string, from, to time.Time) []*Incident {
	s.incidentLock.Lock()
	defer s.incidentLock.Unlock()
	list := []*Incident{}
	for _, i := range s.Incidents {
		if alert != "" && i.AlertKey.Name() != alert {
			continue
		}
		if i.Start.Before(from) || i.Start.After(to) {
			continue
		}
		list = append(list, i)
	}
	return list
}

func (s *Schedule) GetIncidentEvents(id uint64) (*Incident, []Event, []Action, error) {
	s.incidentLock.Lock()
	incident, ok := s.Incidents[id]
	s.incidentLock.Unlock()
	if !ok {
		return nil, nil, nil, fmt.Errorf("incident %d not found", id)
	}
	list := []Event{}
	state := s.GetStatus(incident.AlertKey)
	if state == nil {
		return incident, list, nil, nil
	}
	found := false
	for _, e := range state.History {
		if e.IncidentId == id {
			found = true
			list = append(list, e)
		} else if found {
			break
		}
	}
	actions := []Action{}
	for _, a := range state.Actions {
		if a.Time.After(incident.Start) && (incident.End == nil || a.Time.Before(*incident.End) || a.Time.Equal(*incident.End)) {
			actions = append(actions, a)
		}
	}
	return incident, list, actions, nil
}

func (s *Schedule) Host(filter string) map[string]*HostData {
	s.metalock.Lock()
	res := make(map[string]*HostData)
	for k, mv := range s.Metadata {
		tags := k.TagSet()
		if k.Metric != "" || tags["host"] == "" {
			continue
		}
		e := res[tags["host"]]
		if e == nil {
			host := fmt.Sprintf("{host=%s}", tags["host"])
			e = &HostData{
				Name:       tags["host"],
				Metrics:    s.Search.MetricsByTagPair("host", tags["host"]),
				Interfaces: make(map[string]*HostInterface),
			}
			e.CPU.Processors = make(map[string]string)
			if v, err := s.Search.GetLast("os.cpu", host, true); err != nil {
				e.CPU.Used = v
			}
			e.Memory.Modules = make(map[string]string)
			if v, err := s.Search.GetLast("os.mem.total", host, false); err != nil {
				e.Memory.Total = int64(v)
			}
			if v, err := s.Search.GetLast("os.mem.used", host, false); err != nil {
				e.Memory.Used = int64(v)
			}
			res[tags["host"]] = e
		}
		var iface *HostInterface
		if name := tags["iface"]; name != "" {
			if e.Interfaces[name] == nil {
				h := new(HostInterface)
				itag := opentsdb.TagSet{
					"host":  tags["host"],
					"iface": name,
				}
				intag := opentsdb.TagSet{"direction": "in"}.Merge(itag).String()
				if v, err := s.Search.GetLast("os.net.bytes", intag, true); err != nil {
					h.Inbps = int64(v) * 8
				}
				outtag := opentsdb.TagSet{"direction": "out"}.Merge(itag).String()
				if v, err := s.Search.GetLast("os.net.bytes", outtag, true); err != nil {
					h.Outbps = int64(v) * 8
				}
				e.Interfaces[name] = h
			}
			iface = e.Interfaces[name]
		}
		switch val := mv.Value.(type) {
		case string:
			switch k.Name {
			case "addr":
				if iface != nil {
					iface.IPAddresses = append(iface.IPAddresses, val)
				}
			case "description":
				if iface != nil {
					iface.Description = val
				}
			case "mac":
				if iface != nil {
					iface.MAC = val
				}
			case "manufacturer":
				e.Manufacturer = val
			case "master":
				if iface != nil {
					iface.Master = val
				}
			case "memory":
				if name := tags["name"]; name != "" {
					e.Memory.Modules[name] = val
				}
			case "model":
				e.Model = val
			case "name":
				if iface != nil {
					iface.Name = val
				}
			case "processor":
				if name := tags["name"]; name != "" {
					e.CPU.Processors[name] = val
				}
			case "serialNumber":
				e.SerialNumber = val
			case "version":
				e.OS.Version = val
			case "versionCaption", "uname":
				e.OS.Caption = val
			}
		case float64:
			switch k.Name {
			case "memoryTotal":
				e.Memory.Total = int64(val)
			case "speed":
				if iface != nil {
					iface.LinkSpeed = int64(val)
				}
			}
		}
	}
	s.metalock.Unlock()
	return res
}

type HostInterface struct {
	Description string   `json:",omitempty"`
	IPAddresses []string `json:",omitempty"`
	Inbps       int64    `json:",omitempty"`
	LinkSpeed   int64    `json:",omitempty"`
	MAC         string   `json:",omitempty"`
	Master      string   `json:",omitempty"`
	Name        string   `json:",omitempty"`
	Outbps      int64    `json:",omitempty"`
}

type HostData struct {
	CPU struct {
		Logical    int64             `json:",omitempty"`
		Physical   int64             `json:",omitempty"`
		Used       float64           `json:",omitempty"`
		Processors map[string]string `json:",omitempty"`
	}
	Interfaces   map[string]*HostInterface
	LastBoot     int64  `json:",omitempty"`
	LastUpdate   int64  `json:",omitempty"`
	Manufacturer string `json:",omitempty"`
	Memory       struct {
		Modules map[string]string `json:",omitempty"`
		Total   int64             `json:",omitempty"`
		Used    int64             `json:",omitempty"`
	}
	Metrics []string
	Model   string `json:",omitempty"`
	Name    string `json:",omitempty"`
	OS      struct {
		Caption string `json:",omitempty"`
		Version string `json:",omitempty"`
	}
	SerialNumber string `json:",omitempty"`
}
