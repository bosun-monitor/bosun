package sched

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/StackExchange/tsaf/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/expr"
)

type Schedule struct {
	sync.Mutex

	Conf          *conf.Conf
	status        States
	Notifications map[AlertKey]map[string]time.Time
	Silence       map[string]*Silence
	Group         map[time.Time]AlertKeys

	nc            chan interface{}
	cache         *opentsdb.Cache
	RunHistory    map[AlertKey]*Event
	CheckStart    time.Time
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

func (s *Schedule) MarshalJSON() ([]byte, error) {
	type Grouped struct {
		Active   bool
		Status   Status
		Subject  string
		Len      int
		Alert    string
		AlertKey AlertKey
		Ago      string
		Children []*Grouped `json:",omitempty"`
	}
	t := struct {
		Alerts map[string]*conf.Alert
		Status States
		Groups struct {
			NeedAck      []*Grouped `json:",omitempty"`
			Acknowledged []*Grouped `json:",omitempty"`
		}
		TimeAndDate []int
	}{
		Alerts:      s.Conf.Alerts,
		Status:      make(States),
		TimeAndDate: s.Conf.TimeAndDate,
	}
	s.Lock()
	for k, v := range s.status {
		if !v.Open {
			continue
		}
		t.Status[k] = v
	}
	for tuple, states := range t.Status.GroupStates() {
		var grouped []*Grouped
		switch tuple.Status {
		case StWarning, StCritical:
			for ak, state := range states {
				g := Grouped{
					Active:   tuple.Active,
					Status:   tuple.Status,
					AlertKey: ak,
					Alert:    ak.Name(),
					Subject:  state.Subject,
					Ago:      marshalTime(state.Last().Time),
				}
				grouped = append(grouped, &g)
			}
		case StUnknown:
			for name, group := range states.GroupSets() {
				g := Grouped{
					Active:  tuple.Active,
					Status:  tuple.Status,
					Subject: fmt.Sprintf("%s - %s", tuple.Status, name),
					Len:     len(group),
				}
				for _, ak := range group {
					st := s.status[ak]
					g.Children = append(g.Children, &Grouped{
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
			return nil, fmt.Errorf("unexpected status %v in %v", tuple.Status, tuple)
		}
		if tuple.NeedAck {
			t.Groups.NeedAck = append(t.Groups.NeedAck, grouped...)
		} else {
			t.Groups.Acknowledged = append(t.Groups.Acknowledged, grouped...)
		}
	}
	s.Unlock()
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

func (s *Schedule) Init(c *conf.Conf) {
	s.Conf = c
	s.RunHistory = make(map[AlertKey]*Event)
	s.Silence = make(map[string]*Silence)
	s.Group = make(map[time.Time]AlertKeys)
	s.status = make(States)
	s.cache = opentsdb.NewCache(s.Conf.TsdbHost, s.Conf.ResponseLimit)
}

func (s *Schedule) Load(c *conf.Conf) {
	s.Init(c)
	s.RestoreState()
}

// Restores notification and alert state from the file on disk.
func (s *Schedule) RestoreState() {
	s.Lock()
	defer s.Unlock()
	s.Notifications = nil
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
		if a, present := s.Conf.Alerts[ak.Name()]; !present {
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
			if t == 0 && st.Last().Status == StUnknown {
				st.Append(&Event{Status: StNormal})
			}
		}
		s.status[ak] = &st
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
	for k, v := range s.status {
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

type AlertKey string

func NewAlertKey(name string, group opentsdb.TagSet) AlertKey {
	return AlertKey(name + group.String())
}

func (a AlertKey) Name() string {
	return strings.SplitN(string(a), "{", 2)[0]
}

type AlertKeys []AlertKey

func (a AlertKeys) Len() int           { return len(a) }
func (a AlertKeys) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a AlertKeys) Less(i, j int) bool { return a[i] < a[j] }

type State struct {
	*Result

	// Most recent last.
	History   []Event
	Actions   []Action
	Touched   time.Time
	Alert     string // helper data since AlertKeys don't serialize to JSON well
	Tags      string // string representation of Group
	Group     opentsdb.TagSet
	Subject   string
	NeedAck   bool
	Open      bool
	Forgotten bool
}

func (s *State) AlertKey() AlertKey {
	return NewAlertKey(s.Alert, s.Group)
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

func (s *Schedule) Action(user, message string, t ActionType, ak AlertKey) error {
	s.Lock()
	defer func() {
		s.Unlock()
		s.Save()
	}()
	st := s.status[ak]
	if st == nil {
		return fmt.Errorf("no such alert key: %v", ak)
	}
	switch t {
	case ActionAcknowledge:
		if !st.NeedAck {
			return fmt.Errorf("alert already acknowledged")
		}
		if !st.Open {
			return fmt.Errorf("cannot acknowledge closed alert")
		}
		delete(s.Notifications, ak)
		st.NeedAck = false
	case ActionClose:
		if st.NeedAck {
			return fmt.Errorf("cannot close unacknowledged alert")
		}
		if st.IsActive() {
			return fmt.Errorf("cannot close active alert")
		}
		st.Open = false
	case ActionForget:
		if st.NeedAck {
			return fmt.Errorf("cannot close unacknowledged alert")
		}
		if st.IsActive() {
			return fmt.Errorf("cannot forget active alert")
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
	Warn, Crit *Result
	Status     Status
	Time       time.Time
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
	default:
		return "none"
	}
}

func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

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
