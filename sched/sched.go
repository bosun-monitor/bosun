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
	Freq time.Duration
}

var DefaultSched = Schedule{
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
	s.CheckExpr(a, a.Crit, true)
	s.CheckExpr(a, a.Warn, false)
}

func (s *Schedule) CheckExpr(a *conf.Alert, e *expr.Expr, isCrit bool) {
	if e == nil {
		return
	}
	results, err := e.Execute(s.Conf.TsdbHost)
	if err != nil {
		log.Println(err)
		return
	}
	for _, r := range results {
		if r.Value == 0 {
			continue
		}
		typ := "CRITICAL"
		if !isCrit {
			typ = "WARNING"
		}
		log.Printf("%s: %s, group: %v\n", typ, a.Name, r.Group)
		if !isCrit {
			continue
		}
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
}
