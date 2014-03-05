package sched

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/expr"
)

type Schedule struct {
	sync.Mutex

	Conf          *conf.Conf
	Freq          time.Duration
	Status        map[AlertKey]*State
	Notifications map[AlertKey]map[string]time.Time

	cache *opentsdb.Cache
	nc    chan interface{}
}

func (s *Schedule) MarshalJSON() ([]byte, error) {
	t := struct {
		Alerts map[string]*conf.Alert
		Freq   time.Duration
		Status map[string]*State
	}{
		s.Conf.Alerts,
		s.Freq,
		make(map[string]*State),
	}
	for k, v := range s.Status {
		if v.Last().Status < ST_WARN {
			continue
		}
		t.Status[k.String()] = v
	}
	return json.Marshal(&t)
}

var DefaultSched = &Schedule{
	Freq: time.Minute * 5,
}

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
	for ak, ns := range notifications {
		for name, t := range ns {
			n, present := s.Conf.Notifications[name]
			if !present {
				log.Println("sched: notification not present during restore:", name)
				continue
			}
			_, present = s.Conf.Alerts[ak.Name]
			if !present {
				log.Println("sched: alert not present during restore:", ak.Name)
				continue
			}
			s.AddNotification(ak, n, t)
		}
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
		s.Status[ak] = &st
	}
}

func (s *Schedule) Save() {
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
	go func() {
		for {
			time.Sleep(time.Minute)
			s.Save()
		}
	}()
	s.nc = make(chan interface{}, 1)
	go s.Poll()
	for {
		wait := time.After(s.Freq)
		if s.Freq < time.Second {
			return fmt.Errorf("sched: frequency must be > 1 second")
		}
		if s.Conf == nil {
			return fmt.Errorf("sched: nil configuration")
		}
		start := time.Now()
		s.Check()
		fmt.Printf("run at %v took %v\n", start, time.Since(start))
		<-wait
	}
}

// Poll dispatches notification checks when needed.
func (s *Schedule) Poll() {
	var timeout time.Duration
	for {
		// Wait for one of these two.
		select {
		case <-time.After(timeout):
		case <-s.nc:
		}
		timeout = s.CheckNotifications()
	}
}

// CheckNotifications processes past notification events. It returns the
func (s *Schedule) CheckNotifications() time.Duration {
	s.Lock()
	defer s.Unlock()
	timeout := time.Hour
	notifications := s.Notifications
	s.Notifications = nil
	for ak, ns := range notifications {
		for name, t := range ns {
			n, present := s.Conf.Notifications[name]
			if !present {
				continue
			}
			remaining := t.Add(n.Timeout).Sub(time.Now())
			if remaining > 0 {
				if remaining < timeout {
					timeout = remaining
				}
				s.AddNotification(ak, n, t)
				continue
			}
			st, present := s.Status[ak]
			if !present {
				continue
			}
			a, present := s.Conf.Alerts[ak.Name]
			if !present {
				continue
			}
			s.Notify(st, a, n)
			if n.Timeout < timeout {
				timeout = n.Timeout
			}
		}
	}
	return timeout
}

func (s *Schedule) Check() {
	s.Lock()
	defer s.Unlock()
	s.cache = opentsdb.NewCache(s.Conf.TsdbHost)
	changed := false
	for _, a := range s.Conf.Alerts {
		if s.CheckAlert(a) {
			changed = true
		}
	}
	if changed {
		s.nc <- true
	}
}

func (s *Schedule) CheckAlert(a *conf.Alert) bool {
	crits, cchange := s.CheckExpr(a, a.Crit, true, nil)
	_, wchange := s.CheckExpr(a, a.Warn, false, crits)
	return cchange || wchange
}

func (s *Schedule) CheckExpr(a *conf.Alert, e *expr.Expr, isCrit bool, ignore []AlertKey) (alerts []AlertKey, change bool) {
	if e == nil {
		return
	}
	results, err := e.Execute(s.cache, nil)
	if err != nil {
		// todo: do something here?
		log.Println(err)
		return
	}
Loop:
	for _, r := range results {
		if a.Squelched(r.Group) {
			continue
		}
		ak := AlertKey{a.Name, r.Group.String()}
		for _, v := range ignore {
			if ak == v {
				continue Loop
			}
		}
		state := s.Status[ak]
		if state == nil {
			state = &State{
				Group:        r.Group,
				Computations: r.Computations,
			}
		}
		status := ST_NORM
		if r.Value.(expr.Number) != 0 {
			alerts = append(alerts, ak)
			if isCrit {
				status = ST_CRIT
			} else {
				status = ST_WARN
			}
		}
		state.Expr = e.String()
		var subject = new(bytes.Buffer)
		if err := s.ExecuteSubject(subject, a, state); err != nil {
			log.Println(err)
		}
		state.Subject = subject.String()
		changed := state.Append(status)
		s.Status[ak] = state
		if changed {
			change = true
		}
		if changed && status != ST_NORM {
			notify := func(notifications map[string]*conf.Notification) {
				for _, n := range notifications {
					s.Notify(state, a, n)
				}
			}
			switch status {
			case ST_CRIT:
				notify(a.CritNotification)
			case ST_WARN:
				notify(a.WarnNotification)
			}
		}
	}
	return
}

func (s *Schedule) Notify(st *State, a *conf.Alert, n *conf.Notification) {
	if len(n.Email) > 0 {
		go s.Email(a, n, st)
	}
	if n.Post != nil {
		go s.Post(a, n, st)
	}
	if n.Get != nil {
		go s.Get(a, n, st)
	}
	if n.Print {
		go s.Print(a, n, st)
	}
	if n.Next == nil {
		return
	}
	s.AddNotification(AlertKey{Name: a.Name, Group: st.Group.String()}, n, time.Now().UTC())
}

func (s *Schedule) AddNotification(ak AlertKey, n *conf.Notification, started time.Time) {
	if s.Notifications == nil {
		s.Notifications = make(map[AlertKey]map[string]time.Time)
	}
	if s.Notifications[ak] == nil {
		s.Notifications[ak] = make(map[string]time.Time)
	}
	stn := s.Notifications[ak]
	// Prevent duplicate notifications restarting each other.
	if _, present := stn[n.Name]; !present {
		stn[n.Name] = started
	}
}

type AlertKey struct {
	Name  string
	Group string
}

func (a AlertKey) String() string {
	return a.Name + a.Group
}

type State struct {
	// Most recent event last.
	History      []Event
	Touched      time.Time
	Expr         string
	Group        opentsdb.TagSet
	Computations expr.Computations
	Subject      string
}

func (s *Schedule) Acknowledge(ak AlertKey) {
	s.Lock()
	delete(s.Notifications, ak)
	s.Unlock()
}

func (s *State) Touch() {
	s.Touched = time.Now().UTC()
}

// Appends status to the history if the status is different than the latest
// status. Returns true if state was changed.
func (s *State) Append(status Status) bool {
	s.Touch()
	if len(s.History) == 0 || s.Last().Status != status {
		s.History = append(s.History, Event{status, time.Now().UTC()})
		return true
	}
	return false
}

func (s *State) Last() Event {
	if len(s.History) == 0 {
		return Event{}
	}
	return s.History[len(s.History)-1]
}

type Event struct {
	Status Status
	Time   time.Time // embedding this breaks JSON encoding
}

type Status int

const (
	ST_UNKNOWN Status = iota
	ST_NORM
	ST_WARN
	ST_CRIT
)

func (s Status) String() string {
	switch s {
	case ST_NORM:
		return "normal"
	case ST_WARN:
		return "warning"
	case ST_CRIT:
		return "critical"
	default:
		return "unknown"
	}
}
