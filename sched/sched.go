package sched

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/expr"
)

type Schedule struct {
	*conf.Conf
	sync.Mutex
	Freq   time.Duration
	Status map[AlertKey]*State
	cache  *opentsdb.Cache
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
	s.Status = make(map[AlertKey]*State)
}

func (s *Schedule) Run() error {
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

func (s *Schedule) Check() {
	s.Lock()
	defer s.Unlock()
	s.cache = opentsdb.NewCache(s.Conf.TsdbHost)
	for _, a := range s.Conf.Alerts {
		s.CheckAlert(a)
	}
}

func (s *Schedule) CheckAlert(a *conf.Alert) {
	ignore := s.CheckExpr(a, a.Crit, true, nil)
	s.CheckExpr(a, a.Warn, false, ignore)
}

func (s *Schedule) CheckExpr(a *conf.Alert, e *expr.Expr, isCrit bool, ignore []AlertKey) (alerts []AlertKey) {
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
		status := ST_WARN
		if r.Value.(expr.Number) == 0 {
			status = ST_NORM
		} else if isCrit {
			status = ST_CRIT
		}
		state.Append(status)
		s.Status[ak] = state
		if status != ST_NORM {
			alerts = append(alerts, ak)
			state.Expr = e
			var subject = new(bytes.Buffer)
			a.ExecuteSubject(subject, r.Group, s.cache)
			state.Subject = subject.String()
		}
		if !state.Emailed {
			s.Email(a.Name, r.Group)
		}
	}
	return
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
	Expr         *expr.Expr
	Emailed      bool
	Group        opentsdb.TagSet
	Computations expr.Computations
	Subject      string
}

func (s *State) Touch() {
	s.Touched = time.Now().UTC()
}

// Appends status to the history if the status is different than the latest
// status. Returns true if the status was different.
func (s *State) Append(status Status) {
	s.Touch()
	if len(s.History) == 0 || s.Last().Status != status {
		s.History = append(s.History, Event{status, time.Now().UTC()})
		s.Emailed = status != ST_CRIT
	}
}

func (s *State) Last() Event {
	return s.History[len(s.History)-1]
}

type Event struct {
	Status Status
	Time   time.Time // embedding this breaks JSON encoding
}

type Status int

const (
	ST_NORM Status = iota
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

func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}
