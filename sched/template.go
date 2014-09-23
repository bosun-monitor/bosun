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
	"github.com/StackExchange/bosun/expr/parse"
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

func (c *Context) eval(v string, filter bool, series bool, autods int) ([]*expr.Result, error) {
	e, err := expr.New(v)
	var results []*expr.Result
	if err != nil {
		return results, fmt.Errorf("%s: %v", v, err)
	}
	if series && e.Root.Return() != parse.TYPE_SERIES {
		return results, fmt.Errorf("egraph: requires an expression that returns a series")
	}
	res, _, err := e.Execute(c.schedule.cache, nil, c.schedule.CheckStart, autods, c.Alert.UnjoinedOK, c.schedule.Search, c.schedule.Lookups)
	if err != nil {
		return results, fmt.Errorf("%s: %v", v, err)
	}
	if filter {
		for _, r := range res.Results {
			if r.Group.Equal(c.State.Group) {
				results = append(results, r)
				return results, nil
			}
		}
		for _, r := range res.Results {
			if c.State.Group.Subset(r.Group) {
				results = append(results, r)
				return results, nil
			}
		}
	}
	return res.Results, nil
}

// Eval executes the given expression and returns a value with corresponding tags
// to the context's tags. If no such result is found, the first result with nil
// tags is returned. If no such result is found, "" is returned.
func (c *Context) Eval(v string) interface{} {
	res, err := c.eval(v, true, false, 0)
	if err != nil {
		log.Print(err)
		return ""
	}
	if len(res) != 1 {
		log.Printf("Expected 1 results, got %v", len(res))
		return ""
	}
	return res[0].Value
}

// EvalAll executes the expression and returns the result set. It is not filtered
// to the context's tags.
func (c *Context) EvalAll(v string) interface{} {
	res, err := c.eval(v, false, false, 0)
	if err != nil {
		log.Print(err)
		return ""
	}
	return res
}

func (c *Context) graph(v string, res []*expr.Result) interface{} {
	var buf bytes.Buffer
	if err := c.schedule.ExprGraph(nil, &buf, res, v, time.Now().UTC()); err != nil {
		return err.Error()
	}
	return template.HTML(buf.String())
}

func (c *Context) Graph(v string) interface{} {
	res, err := c.eval(v, false, true, 1000)
	if err != nil {
		log.Print(err)
		return ""
	}
	return c.graph(v, res)
}

func (c *Context) GraphAll(v string) interface{} {
	res, err := c.eval(v, true, true, 1000)
	if err != nil {
		log.Print(err)
		return ""
	}
	return c.graph(v, res)
}
