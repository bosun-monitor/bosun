package sched

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/StackExchange/tsaf/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/expr"
)

type Context struct {
	*State
	Alert *conf.Alert

	Schedule *Schedule
}

func (s *Schedule) data(st *State, a *conf.Alert) *Context {
	return &Context{
		State:    st,
		Alert:    a,
		Schedule: s,
	}
}

type unknownContext struct {
	Time  time.Time
	Name  string
	Group AlertKeys

	schedule *Schedule
}

func (s *Schedule) unknownData(t time.Time, name string, group AlertKeys) *unknownContext {
	return &unknownContext{
		Time:     t,
		Group:    group,
		Name:     name,
		schedule: s,
	}
}

// URL returns a prepopulated URL for external access, with path and query empty.
func (s *Schedule) URL() *url.URL {
	u := url.URL{
		Scheme: "http",
		Host:   s.Conf.HttpListen,
	}
	if strings.HasPrefix(s.Conf.HttpListen, ":") {
		h, err := os.Hostname()
		if err != nil {
			u.Host = "localhost" + u.Host
		} else {
			u.Host = h + u.Host
		}
	}
	return &u
}

// Ack returns the URL to acknowledge an alert.
func (c *Context) Ack() string {
	u := c.Schedule.URL()
	u.Path = fmt.Sprintf("/api/acknowledge/%s/%s", c.Alert.Name, c.State.Group.String())
	return u.String()
}

// HostView returns the URL to the host view page.
func (c *Context) HostView(host string) string {
	u := c.Schedule.URL()
	u.Path = "/host"
	u.RawQuery = fmt.Sprintf("time=1d-ago&host=%s", host)
	return u.String()
}

func (c *Context) EGraph(v string) string {
	q := url.QueryEscape("q=" + opentsdb.ReplaceTags(v, c.Group))
	u := url.URL{
		Scheme:   "http",
		Host:     c.Schedule.Conf.HttpListen,
		Path:     "/egraph",
		RawQuery: q,
	}
	if strings.HasPrefix(c.Schedule.Conf.HttpListen, ":") {
		h, err := os.Hostname()
		if err != nil {
			return ""
		}
		u.Host = h + u.Host
	}
	return u.String()
}

func (s *Schedule) ExecuteBody(w io.Writer, a *conf.Alert, st *State) error {
	t := a.Template
	if t == nil || t.Body == nil {
		return nil
	}
	return t.Body.Execute(w, s.data(st, a))
}

func (s *Schedule) ExecuteSubject(w io.Writer, a *conf.Alert, st *State) error {
	t := a.Template
	if t == nil || t.Subject == nil {
		return nil
	}
	return t.Subject.Execute(w, s.data(st, a))
}

// E executes the given expression and returns a value with corresponding tags
// to the Context's tags. If no such result is found, the first result with nil
// tags is returned. If no such result is found, nil is returned. The precision
// of numbers is truncated for convienent display. Array expressions are not
// supported.
func (c *Context) E(v string) (s string) {
	e, err := expr.New(v)
	if err != nil {
		log.Printf("%s: %v", v, err)
		return
	}
	res, _, err := e.Execute(c.Schedule.cache, nil)
	if err != nil {
		log.Printf("%s: %v", v, err)
		return
	}
	for _, r := range res {
		if r.Group.Equal(c.State.Group) {
			s = truncate(r.Value)
		}
	}
	for _, r := range res {
		if r.Group == nil {
			s = truncate(r.Value)
		}
	}
	return
}

// truncate displays needed decimals for a Number.
func truncate(v expr.Value) string {
	switch t := v.(type) {
	case expr.Number:
		if t < 1 {
			return fmt.Sprintf("%.4f", t)
		} else if t < 100 {
			return fmt.Sprintf("%.1f", t)
		} else {
			return fmt.Sprintf("%.0f", t)
		}
	default:
		return ""
	}
}
