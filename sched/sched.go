package sched

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/StackExchange/tsaf/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/expr"
)

type Schedule struct {
	sync.Mutex

	Conf          *conf.Conf
	Status        States
	Notifications map[AlertKey]map[string]time.Time
	Silence       map[string]*Silence
	Group         map[time.Time]AlertKeys

	nc            chan interface{}
	cache         *opentsdb.Cache
	runStates     map[AlertKey]Status
	notifications map[*conf.Notification][]*State
}

type States map[AlertKey]*State

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
			st.Last().Status != stNormal,
			st.Last().Status,
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
func (states States) GroupSets() map[string]AlertKeys {
	type Pair struct {
		k, v string
	}
	groups := make(map[string]AlertKeys)
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
		var group AlertKeys
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
		var group AlertKeys
		for _, s := range states {
			if seen[s] {
				continue
			}
			if group == nil || s.AlertKey().Name == group[0].Name {
				group = append(group, s.AlertKey())
				seen[s] = true
			}
		}
		if len(group) > 0 {
			groups[group[0].Name] = group
		}
	}
	return groups
}

func (s *Schedule) MarshalJSON() ([]byte, error) {
	type Grouped struct {
		Active   bool
		Status   Status
		Subject  string
		Len      int
		Key      string `json:",omitempty"`
		AlertKey AlertKey
		Ago      string
		Children []*Grouped `json:",omitempty"`
	}
	t := struct {
		Alerts map[string]*conf.Alert
		Status map[string]*State
		Groups struct {
			NeedAck      []*Grouped `json:",omitempty"`
			Acknowledged []*Grouped `json:",omitempty"`
		}
		TimeAndDate []int
	}{
		Alerts:      s.Conf.Alerts,
		Status:      make(map[string]*State),
		TimeAndDate: s.Conf.TimeAndDate,
	}
	for k, v := range s.Status {
		if v.Last().Status < stWarning {
			continue
		}
		t.Status[k.String()] = v
	}
	for tuple, states := range s.Status.GroupStates() {
		var grouped []*Grouped
		switch tuple.Status {
		case stWarning, stCritical:
			for ak, state := range states {
				g := Grouped{
					Active:   tuple.Active,
					Status:   tuple.Status,
					AlertKey: ak,
					Key:      ak.String(),
					Subject:  state.Subject,
					Ago:      marshalTime(state.Last().Time),
				}
				grouped = append(grouped, &g)
			}
		case stUnknown:
			for name, group := range states.GroupSets() {
				g := Grouped{
					Active:  tuple.Active,
					Status:  tuple.Status,
					Subject: fmt.Sprintf("%s - %s", tuple.Status, name),
					Len:     len(group),
				}
				for _, ak := range group {
					st := s.Status[ak]
					g.Children = append(g.Children, &Grouped{
						Active:   tuple.Active,
						Status:   tuple.Status,
						AlertKey: ak,
						Key:      ak.String(),
						Subject:  st.Subject,
						Ago:      marshalTime(st.Last().Time),
					})
				}
				grouped = append(grouped, &g)
			}
		}
		if tuple.NeedAck {
			t.Groups.NeedAck = append(t.Groups.NeedAck, grouped...)
		} else {
			t.Groups.Acknowledged = append(t.Groups.Acknowledged, grouped...)
		}
	}
	return json.Marshal(&t)
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

func (s *Schedule) Load(c *conf.Conf) {
	s.Conf = c
	s.Silence = make(map[string]*Silence)
	s.Group = make(map[time.Time]AlertKeys)
	s.RestoreState()
}

// Restores notification and alert state from the file on disk.
func (s *Schedule) RestoreState() {
	s.Lock()
	defer s.Unlock()
	s.cache = opentsdb.NewCache(s.Conf.TsdbHost)
	s.Notifications = nil
	s.Status = make(map[AlertKey]*State)
	f, err := os.Open(s.Conf.StateFile)
	if err != nil {
		log.Println(err)
		return
	}
	dec := gob.NewDecoder(f)
	notifications := make(map[AlertKey]map[string]time.Time)
	if err := dec.Decode(&notifications); err != nil {
		log.Println(err)
		return
	}
	if err := dec.Decode(&s.Silence); err != nil {
		log.Println(err)
		return
	}
	for {
		var ak AlertKey
		var st State
		if err := dec.Decode(&ak); err == io.EOF {
			break
		} else if err != nil {
			log.Println(err)
			return
		}
		if err := dec.Decode(&st); err != nil {
			log.Println(err)
			return
		}
		if a, present := s.Conf.Alerts[ak.Name]; !present {
			log.Println("sched: alert no longer present, ignoring:", ak)
			continue
		} else if a.Squelched(st.Group) {
			log.Println("sched: alert now squelched:", ak)
			continue
		} else {
			t := a.Unknown
			if t == 0 {
				t = s.Conf.Unknown
			}
			if t == 0 && st.Last().Status == stUnknown {
				st.Append(stNormal)
			}
		}
		s.Status[ak] = &st
		for name, t := range notifications[ak] {
			n, present := s.Conf.Notifications[name]
			if !present {
				log.Println("sched: notification not present during restore:", name)
				continue
			}
			s.AddNotification(ak, n, t)
		}
	}
}

func (s *Schedule) Save() {
	// todo: debounce this call
	go s.save()
}

func (s *Schedule) save() {
	s.Lock()
	defer s.Unlock()
	f, err := os.Create(s.Conf.StateFile)
	if err != nil {
		log.Println(err)
		return
	}
	enc := gob.NewEncoder(f)
	if err := enc.Encode(s.Notifications); err != nil {
		log.Println(err)
		return
	}
	enc.Encode(s.Silence)
	for k, v := range s.Status {
		enc.Encode(k)
		enc.Encode(v)
	}
	if err := f.Close(); err != nil {
		log.Println(err)
		return
	}
	log.Println("sched: wrote state to", s.Conf.StateFile)
}

func (s *Schedule) Run() error {
	s.nc = make(chan interface{}, 1)
	go s.Poll()
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
		s.Check()
		log.Printf("run at %v took %v\n", start, time.Since(start))
		<-wait
	}
}

type AlertKey struct {
	Name  string
	Group string
}

func NewAlertKey(name string, group opentsdb.TagSet) AlertKey {
	return AlertKey{
		Name:  name,
		Group: group.String(),
	}
}

func (a AlertKey) String() string {
	return a.Name + a.Group
}

type AlertKeys []AlertKey

func (a AlertKeys) Len() int      { return len(a) }
func (a AlertKeys) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a AlertKeys) Less(i, j int) bool {
	if a[i].Name == a[j].Name {
		return a[i].Group < a[j].Group
	}
	return a[i].Name < a[j].Name
}

type State struct {
	// Most recent event last.
	History      []Event
	Touched      time.Time
	Expr         string
	Alert        string // helper data since AlertKeys don't serialize to JSON well
	Tags         string // string representation of Group
	Group        opentsdb.TagSet
	Computations expr.Computations
	Subject      string
	NeedAck      bool
}

func (s *State) AlertKey() AlertKey {
	return NewAlertKey(s.Alert, s.Group)
}

func (s *Schedule) Acknowledge(ak AlertKey) {
	s.Lock()
	delete(s.Notifications, ak)
	if st := s.Status[ak]; st != nil {
		st.NeedAck = false
	}
	s.Unlock()
	s.Save()
}

func (s *State) Touch() {
	s.Touched = time.Now().UTC()
}

// Appends status to the history if the status is different than the latest
// status. Returns the previous status.
func (s *State) Append(status Status) Status {
	last := s.Last()
	if len(s.History) == 0 || s.Last().Status != status {
		s.History = append(s.History, Event{status, time.Now().UTC()})
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
	Status Status
	Time   time.Time
}

type Status int

const (
	stNone Status = iota
	stNormal
	stWarning
	stCritical
	stUnknown
)

func (s Status) String() string {
	switch s {
	case stNormal:
		return "normal"
	case stWarning:
		return "warning"
	case stCritical:
		return "critical"
	case stUnknown:
		return "unknown"
	default:
		return "none"
	}
}

func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}
