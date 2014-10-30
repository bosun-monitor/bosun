package sched

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/collect"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/bosun/_third_party/github.com/bradfitz/slice"
	"github.com/StackExchange/bosun/_third_party/github.com/tatsushid/go-fastping"
	"github.com/StackExchange/bosun/conf"
	"github.com/StackExchange/bosun/expr"
	"github.com/StackExchange/bosun/search"
)

func init() {
	gob.Register(expr.Number(0))
	gob.Register(expr.Scalar(0))
}

type Schedule struct {
	sync.Mutex

	Conf          *conf.Conf
	status        States
	Notifications map[expr.AlertKey]map[string]time.Time
	Silence       map[string]*Silence
	Group         map[time.Time]expr.AlertKeys
	Metadata      map[metadata.Metakey]Metavalues
	Search        *search.Search
	Lookups       map[string]*expr.Lookup

	nc            chan interface{}
	cache         *opentsdb.Cache
	CheckStart    time.Time
	notifications map[*conf.Notification][]*State
	metalock      sync.Mutex
}

type Metavalues []Metavalue

func (m Metavalues) Last() *Metavalue {
	if len(m) > 0 {
		return &m[len(m)-1]
	}
	return nil
}

type Metavalue struct {
	Time  time.Time
	Value interface{}
}

func (s *Schedule) PutMetadata(k metadata.Metakey, v interface{}) {
	s.metalock.Lock()
	md := s.Metadata[k]
	mv := Metavalue{time.Now().UTC(), v}
	changed := false
	if md == nil {
		changed = true
	} else {
		last := md[len(md)-1]
		changed = !reflect.DeepEqual(last.Value, v)
	}
	if changed {
		s.Metadata[k] = append(md, mv)
	} else {
		s.Metadata[k][len(md)-1].Time = mv.Time
	}
	s.Save()
	s.metalock.Unlock()
}

func (s *Schedule) GetMetadata(metric string, subset opentsdb.TagSet) []metadata.Metasend {
	s.metalock.Lock()
	ms := make([]metadata.Metasend, 0)
	for k, v := range s.Metadata {
		if metric != "" && k.Metric != metric {
			continue
		}
		if !k.TagSet().Subset(subset) {
			continue
		}
		mv := v.Last()
		if mv == nil {
			continue
		}
		ms = append(ms, metadata.Metasend{
			Metric: k.Metric,
			Tags:   k.TagSet(),
			Name:   k.Name,
			Value:  mv.Value,
			Time:   mv.Time,
		})
	}
	s.metalock.Unlock()
	return ms
}

type States map[expr.AlertKey]*State

type StateTuple struct {
	NeedAck bool
	Active  bool
	Status  Status
}

// GroupStates groups by NeedAck, Active, and Status.
func (s States) GroupStates() map[StateTuple]States {
	r := make(map[StateTuple]States)
	for ak, st := range s {
		t := StateTuple{
			st.NeedAck,
			st.IsActive(),
			st.AbnormalStatus(),
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

type StateGroup struct {
	Active   bool `json:",omitempty"`
	Status   Status
	Subject  string        `json:",omitempty"`
	Len      int           `json:",omitempty"`
	Alert    string        `json:",omitempty"`
	AlertKey expr.AlertKey `json:",omitempty"`
	Ago      string        `json:",omitempty"`
	Children []*StateGroup `json:",omitempty"`
}

type StateGroups struct {
	Groups struct {
		NeedAck      []*StateGroup `json:",omitempty"`
		Acknowledged []*StateGroup `json:",omitempty"`
	}
	TimeAndDate []int
	Silenced    map[expr.AlertKey]time.Time
}

func (s *Schedule) MarshalGroups(filter string) (*StateGroups, error) {
	t := StateGroups{
		TimeAndDate: s.Conf.TimeAndDate,
		Silenced:    s.Silenced(),
	}
	s.Lock()
	defer s.Unlock()
	status := make(States)
	matches, err := makeFilter(filter)
	if err != nil {
		return nil, err
	}
	for k, v := range s.status {
		if !v.Open {
			continue
		}
		a := s.Conf.Alerts[k.Name()]
		if a == nil {
			return nil, fmt.Errorf("unknown alert %s", k.Name())
		}
		if matches(s.Conf, a, v) {
			status[k] = v
		}
	}
	for tuple, states := range status.GroupStates() {
		var grouped []*StateGroup
		switch tuple.Status {
		case StWarning, StCritical, StUnknown, StError:
			for name, group := range states.GroupSets() {
				g := StateGroup{
					Active:  tuple.Active,
					Status:  tuple.Status,
					Subject: fmt.Sprintf("%s - %s", tuple.Status, name),
					Len:     len(group),
				}
				for _, ak := range group {
					st := s.status[ak]
					g.Children = append(g.Children, &StateGroup{
						Active:   tuple.Active,
						Status:   tuple.Status,
						AlertKey: ak,
						Alert:    ak.Name(),
						Subject:  st.Subject,
						Ago:      marshalTime(st.Last().Time),
					})
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

// Loads a configuration into the default schedule
func Load(c *conf.Conf) {
	DefaultSched.Load(c)
}

// Runs the default schedule.
func Run() error {
	return DefaultSched.Run()
}

func (s *Schedule) Init(c *conf.Conf) {
	s.Conf = c
	s.Silence = make(map[string]*Silence)
	s.Group = make(map[time.Time]expr.AlertKeys)
	s.Metadata = make(map[metadata.Metakey]Metavalues)
	s.Lookups = c.GetLookups()
	s.status = make(States)
	s.cache = opentsdb.NewCache(s.Conf.TsdbHost, s.Conf.ResponseLimit)
	s.Search = search.NewSearch()
}

func (s *Schedule) Load(c *conf.Conf) {
	s.Init(c)
	s.RestoreState()
}

// Restores notification and alert state from the file on disk.
func (s *Schedule) RestoreState() {
	s.Lock()
	defer s.Unlock()
	s.Search.Lock()
	defer s.Search.Unlock()
	s.Notifications = nil
	f, err := os.Open(s.Conf.StateFile)
	if err != nil {
		log.Println(err)
		return
	}
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&s.Search.Metric); err != nil {
		log.Println(err)
		return
	}
	if err := dec.Decode(&s.Search.Tagk); err != nil {
		log.Println(err)
		return
	}
	if err := dec.Decode(&s.Search.Tagv); err != nil {
		log.Println(err)
		return
	}
	if err := dec.Decode(&s.Search.MetricTags); err != nil {
		log.Println(err)
		return
	}
	notifications := make(map[expr.AlertKey]map[string]time.Time)
	if err := dec.Decode(&notifications); err != nil {
		log.Println(err)
		return
	}
	if err := dec.Decode(&s.Silence); err != nil {
		log.Println(err)
		return
	}
	status := make(States)
	if err := dec.Decode(&status); err != nil {
		log.Println(err)
		return
	}
	for ak, st := range status {
		if a, present := s.Conf.Alerts[ak.Name()]; !present {
			log.Println("sched: alert no longer present, ignoring:", ak)
			continue
		} else if s.Conf.Squelched(a, st.Group) {
			log.Println("sched: alert now squelched:", ak)
			continue
		} else if st.Status().IsUnknown() && a.IgnoreUnknown {
			log.Println("sched: alert now disregards unknown:", ak)
			continue
		} else {
			t := a.Unknown
			if t == 0 {
				t = s.Conf.CheckFrequency
			}
			if t == 0 && st.Last().Status == StUnknown {
				st.Append(&Event{Status: StNormal})
			}
		}
		s.status[ak] = st
		for name, t := range notifications[ak] {
			n, present := s.Conf.Notifications[name]
			if !present {
				log.Println("sched: notification not present during restore:", name)
				continue
			}
			s.AddNotification(ak, n, t)
		}
	}
	if err := dec.Decode(&s.Metadata); err != nil {
		log.Println(err)
		return
	}
}

var savePending bool

func (s *Schedule) Save() {
	go func() {
		s.Lock()
		defer s.Unlock()
		if savePending {
			return
		}
		savePending = true
		time.AfterFunc(time.Second*5, s.save)
	}()
}

func (s *Schedule) save() {
	s.Lock()
	s.Search.Lock()
	defer s.Search.Unlock()
	defer s.Unlock()
	savePending = false
	if s.Conf.StateFile == "" {
		return
	}
	tmp := s.Conf.StateFile + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		log.Println(err)
		return
	}
	enc := gob.NewEncoder(f)
	if err := enc.Encode(s.Search.Metric); err != nil {
		log.Println(err)
		return
	}
	if err := enc.Encode(s.Search.Tagk); err != nil {
		log.Println(err)
		return
	}
	if err := enc.Encode(s.Search.Tagv); err != nil {
		log.Println(err)
		return
	}
	if err := enc.Encode(s.Search.MetricTags); err != nil {
		log.Println(err)
		return
	}
	if err := enc.Encode(s.Notifications); err != nil {
		log.Println(err)
		return
	}
	if err := enc.Encode(s.Silence); err != nil {
		log.Println(err)
		return
	}
	if err := enc.Encode(s.status); err != nil {
		log.Println(err)
		return
	}
	if err := enc.Encode(s.Metadata); err != nil {
		log.Println(err)
		return
	}
	if err := f.Close(); err != nil {
		log.Println(err)
		return
	}
	if err := os.Rename(tmp, s.Conf.StateFile); err != nil {
		log.Println(err)
		return
	}
	log.Println("sched: wrote state to", s.Conf.StateFile)
}

func (s *Schedule) Run() error {
	s.nc = make(chan interface{}, 1)
	if s.Conf.Ping {
		go s.PingHosts()
	}
	go s.Poll()
	go s.CheckUnknown()
	for {
		wait := time.After(s.Conf.CheckFrequency)
		if s.Conf.CheckFrequency < time.Second {
			return fmt.Errorf("sched: frequency must be > 1 second")
		}
		if s.Conf == nil {
			return fmt.Errorf("sched: nil configuration")
		}
		start := time.Now()
		log.Printf("starting run at %v\n", start)
		s.Check(nil, start.UTC())
		log.Printf("run at %v took %v\n", start, time.Since(start))
		<-wait
	}
}

const pingFreq = time.Second * 15

func (s *Schedule) PingHosts() {
	for _ = range time.Tick(pingFreq) {
		hosts := s.Search.TagValuesByTagKey("host")
		for _, host := range hosts {
			go pingHost(host)
		}
	}
}

func pingHost(host string) {
	p := fastping.NewPinger()
	ra, err := net.ResolveIPAddr("ip4:icmp", host)
	if err != nil {
		collect.Put("ping.resolved", opentsdb.TagSet{"dst_host": host}, 0)
		return
	}
	collect.Put("ping.resolved", opentsdb.TagSet{"dst_host": host}, 1)
	p.AddIPAddr(ra)
	p.MaxRTT = time.Second * 5
	timeout := 1
	p.AddHandler("receive", func(addr *net.IPAddr, t time.Duration) {
		collect.Put("ping.rtt", opentsdb.TagSet{"dst_host": host}, float64(t)/float64(time.Millisecond))
		timeout = 0
	})
	if err := p.Run(); err != nil {
		log.Print(err)
	}
	collect.Put("ping.timeout", opentsdb.TagSet{"dst_host": host}, timeout)
}

type State struct {
	*Result

	// Most recent last.
	History   []Event  `json:",omitempty"`
	Actions   []Action `json:",omitempty"`
	Touched   time.Time
	Alert     string // helper data since AlertKeys don't serialize to JSON well
	Tags      string // string representation of Group
	Group     opentsdb.TagSet
	Subject   string
	NeedAck   bool
	Open      bool
	Forgotten bool
}

func (s *State) AlertKey() expr.AlertKey {
	return expr.NewAlertKey(s.Alert, s.Group)
}

func (s *State) Status() Status {
	return s.Last().Status
}

// AbnormalStatus returns the most recent non-normal status, or StNone if none
// found.
func (s *State) AbnormalStatus() Status {
	for i := len(s.History) - 1; i >= 0; i-- {
		if st := s.History[i].Status; st > StNormal {
			return st
		}
	}
	return StNone
}

func (s *State) IsActive() bool {
	return s.Status() > StNormal
}

func (s *Schedule) Action(user, message string, t ActionType, ak expr.AlertKey) error {
	s.Lock()
	defer func() {
		s.Unlock()
		s.Save()
	}()
	st := s.status[ak]
	if st == nil {
		return fmt.Errorf("no such alert key: %v", ak)
	}
	ack := func() {
		delete(s.Notifications, ak)
		st.NeedAck = false
	}
	isUnknown := st.Last().Status == StUnknown
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
		if st.IsActive() {
			return fmt.Errorf("cannot close active alert")
		}
		st.Open = false
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
	st.Actions = append(st.Actions, Action{
		User:    user,
		Message: message,
		Type:    t,
		Time:    time.Now().UTC(),
	})
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

// Appends status to the history if the status is different than the latest
// status. Returns the previous status.
func (s *State) Append(event *Event) Status {
	last := s.Last()
	if len(s.History) == 0 || s.Last().Status != event.Status {
		event.Time = time.Now().UTC()
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
}

type Result struct {
	*expr.Result
	Expr string
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
