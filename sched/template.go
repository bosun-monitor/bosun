package sched

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/bosun/conf"
	"github.com/StackExchange/bosun/expr"
)

type Context struct {
	*State
	Alert *conf.Alert

	schedule *Schedule
}

func (s *Schedule) Data(st *State, a *conf.Alert) *Context {
	return &Context{
		State:    st,
		Alert:    a,
		schedule: s,
	}
}

type unknownContext struct {
	Time  time.Time
	Name  string
	Group expr.AlertKeys

	schedule *Schedule
}

func (s *Schedule) unknownData(t time.Time, name string, group expr.AlertKeys) *unknownContext {
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
	u := c.schedule.URL()
	u.Path = "/action"
	u.RawQuery = url.Values{
		"type": []string{"ack"},
		"key":  []string{c.Alert.Name + c.State.Group.String()},
	}.Encode()
	return u.String()
}

// HostView returns the URL to the host view page.
func (c *Context) HostView(host string) string {
	u := c.schedule.URL()
	u.Path = "/host"
	u.RawQuery = fmt.Sprintf("time=1d-ago&host=%s", host)
	return u.String()
}

func (c *Context) Expr(v string) string {
	q := url.QueryEscape("q=" + opentsdb.ReplaceTags(v, c.Group))
	u := url.URL{
		Scheme:   "http",
		Host:     c.schedule.Conf.HttpListen,
		Path:     "/expr",
		RawQuery: q,
	}
	if strings.HasPrefix(c.schedule.Conf.HttpListen, ":") {
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
	return t.Body.Execute(w, s.Data(st, a))
}

func (s *Schedule) ExecuteSubject(w io.Writer, a *conf.Alert, st *State) error {
	t := a.Template
	if t == nil || t.Subject == nil {
		return nil
	}
	return t.Subject.Execute(w, s.Data(st, a))
}

// E executes the given expression and returns a value with corresponding tags
// to the context's tags. If no such result is found, the first result with nil
// tags is returned. If no such result is found, "" is returned. The precision
// of numbers is truncated for convienent display. Series expressions are not
// supported.
func (c *Context) E(v string) string {
	e, err := expr.New(v)
	if err != nil {
		log.Printf("%s: %v", v, err)
		return ""
	}
	res, _, err := e.Execute(c.schedule.cache, nil, c.schedule.CheckStart, 0, c.Alert.UnjoinedOK, c.schedule.Search, c.schedule.Lookups)
	if err != nil {
		log.Printf("%s: %v", v, err)
		return ""
	}
	for _, r := range res.Results {
		if r.Group.Equal(c.State.Group) {
			return truncate(r.Value)
		}
	}
	for _, r := range res.Results {
		if c.State.Group.Subset(r.Group) {
			return truncate(r.Value)
		}
	}
	for _, r := range res.Results {
		if r.Group == nil {
			return truncate(r.Value)
		}
	}
	return ""
}

// ENC executes the given epression and returns the full set of values outside
// of the context of the instance. It is useful for when you want to provide
// more detailed information in the notification that the scope of the alert trigger
func (c *Context) ENC(v string) interface{} {
	e, err := expr.New(v)
	if err != nil {
		log.Printf("%s: %v", v, err)
		return ""
	}
	res, _, err := e.Execute(c.schedule.cache, nil, c.schedule.CheckStart, 0, c.Alert.UnjoinedOK, c.schedule.Search, c.schedule.Lookups)
	if err != nil {
		log.Printf("%s: %v", v, err)
		return ""
	}
	return res.Results
}

func (c *Context) Graph(v string) interface{} {
	var buf bytes.Buffer
	if err := c.schedule.ExprGraph(nil, &buf, v, time.Now().UTC(), 1000); err != nil {
		return err.Error()
	}
	return template.HTML(buf.String())
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
