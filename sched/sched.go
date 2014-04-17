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

	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/expr"
	"github.com/StackExchange/tsaf/third_party/github.com/StackExchange/scollector/opentsdb"
)

type Schedule struct {
	sync.Mutex

	Conf          *conf.Conf
	Status        map[AlertKey]*State
	Notifications map[AlertKey]map[string]time.Time
	Silence       map[string]*Silence
	Group         map[time.Time]AlertKeys

	nc            chan interface{}
	cache         *opentsdb.Cache
	runStates     map[AlertKey]Status
	notifications map[*conf.Notification][]*State
}

func (s *Schedule) MarshalJSON() ([]byte, error) {
	t := struct {
		Alerts      map[string]*conf.Alert
		Status      map[string]*State
		TimeAndDate []int
	}{
		s.Conf.Alerts,
		make(map[string]*State),
		s.Conf.TimeAndDate,
	}
	for k, v := range s.Status {
		if v.Last().Status < stWarning {
			continue
		}
		t.Status[k.String()] = v
	}
	return json.Marshal(&t)
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
	//go s.save()
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
