package sched

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jordan-wright/email"

	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/expr"
)

type Schedule struct {
	*conf.Conf
	Freq   time.Duration
	Status map[AlertKey]*State
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
		s.Check()
		start := time.Now()
		fmt.Printf("run at %v took %v\n", start, time.Since(start))
		<-wait
	}
}

func (s *Schedule) Check() {
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
	results, err := e.Execute(s.Conf.TsdbHost)
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
			state = new(State)
		}
		status := ST_WARN
		if r.Value == 0 {
			status = ST_NORM
		} else if isCrit {
			status = ST_CRIT
		}
		state.Append(status)
		s.Status[ak] = state
		if status != ST_NORM {
			alerts = append(alerts, ak)
		}
		if status != ST_CRIT {
			continue
		}
		continue
		body := new(bytes.Buffer)
		subject := new(bytes.Buffer)
		data := struct {
			Alert *conf.Alert
			Tags  opentsdb.TagSet
		}{
			a,
			r.Group,
		}
		if a.Template.Body != nil {
			err := a.Template.Body.Execute(body, &data)
			if err != nil {
				log.Println(err)
				continue
			}
		}
		if a.Template.Subject != nil {
			err := a.Template.Subject.Execute(subject, &data)
			if err != nil {
				log.Println(err)
				continue
			}
		}
		if a.Owner != "" {
			e := email.NewEmail()
			e.From = "tsaf@stackexchange.com"
			e.To = strings.Split(a.Owner, ",")
			e.Subject = subject.String()
			e.Text = body.String()
			err := e.Send("ny-mail:25", nil)
			if err != nil {
				log.Println(err)
			}
		}
	}
	return
}

type AlertKey struct {
	Name  string
	Group string
}

type State struct {
	// Most recent event last.
	History []Event
	Touched time.Time
}

func (s *State) Touch() {
	s.Touched = time.Now().UTC()
}

// Appends status to the history if the status is different than the latest
// status.
func (s *State) Append(status Status) {
	s.Touch()
	if len(s.History) == 0 || s.Last().Status != status {
		s.History = append(s.History, Event{status, time.Now().UTC()})
	}
}

func (s *State) Last() Event {
	return s.History[len(s.History)-1]
}

type Event struct {
	Status
	time.Time
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
