package sched

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"

	"io"
	"math"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/bosun-monitor/bosun/_third_party/github.com/bosun-monitor/scollector/opentsdb"

	"github.com/bosun-monitor/bosun/conf"
	"github.com/bosun-monitor/bosun/expr"
	"github.com/bosun-monitor/bosun/expr/parse"
)

type Context struct {
	*State
	Alert *conf.Alert

	schedule    *Schedule
	runHistory  *RunHistory
	Attachments []*conf.Attachment
}

func (s *Schedule) Data(rh *RunHistory, st *State, a *conf.Alert, isEmail bool) *Context {
	c := Context{
		State:      st,
		Alert:      a,
		schedule:   s,
		runHistory: rh,
	}
	if isEmail {
		c.Attachments = make([]*conf.Attachment, 0)
	}
	return &c
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

func (c *Context) makeLink(path string, v *url.Values) (string, error) {
	u := url.URL{
		Scheme:   "http",
		Host:     c.schedule.Conf.HttpListen,
		Path:     path,
		RawQuery: v.Encode(),
	}
	if strings.HasPrefix(c.schedule.Conf.HttpListen, ":") {
		h, err := os.Hostname()
		if err != nil {
			return "", err
		}
		u.Host = h + u.Host
	}
	return u.String(), nil
}

func (c *Context) Expr(v string) (string, error) {
	p := url.Values{}
	p.Add("expr", base64.StdEncoding.EncodeToString([]byte(opentsdb.ReplaceTags(v, c.Group))))
	return c.makeLink("/expr", &p)
}

func (c *Context) Rule() (string, error) {
	t, err := c.schedule.Conf.AlertTemplateStrings()
	if err != nil {
		return "", err
	}
	p := url.Values{}
	adef := base64.StdEncoding.EncodeToString([]byte(t.Alerts[c.Alert.Name]))
	tdef := base64.StdEncoding.EncodeToString([]byte(t.Templates[c.Alert.Template.Name]))
	p.Add("alert", adef)
	p.Add("template", tdef)
	return c.makeLink("/rule", &p)
}

func (s *Schedule) ExecuteBody(w io.Writer, rh *RunHistory, a *conf.Alert, st *State, isEmail bool) ([]*conf.Attachment, error) {
	t := a.Template
	if t == nil || t.Body == nil {
		return nil, nil
	}
	c := s.Data(rh, st, a, isEmail)
	return c.Attachments, t.Body.Execute(w, c)
}

func (s *Schedule) ExecuteSubject(w io.Writer, rh *RunHistory, a *conf.Alert, st *State) error {
	t := a.Template
	if t == nil || t.Subject == nil {
		return nil
	}
	return t.Subject.Execute(w, s.Data(rh, st, a, false))
}

func (c *Context) eval(v interface{}, filter bool, series bool, autods int) ([]*expr.Result, string, error) {
	var e *expr.Expr
	var err error
	switch v := v.(type) {
	case string:
		e, err = expr.New(v)
	case *expr.Expr:
		e = v
	default:
		return nil, "", fmt.Errorf("expected string or expression, got %T (%v)", v, v)
	}
	if err != nil {
		return nil, "", fmt.Errorf("%v: %v", v, err)
	}
	if filter {
		e, err = expr.New(opentsdb.ReplaceTags(e.String(), c.State.Group))
		if err != nil {
			return nil, "", err
		}
	}
	if series && e.Root.Return() != parse.TYPE_SERIES {
		return nil, "", fmt.Errorf("egraph: requires an expression that returns a series")
	}
	res, _, err := e.Execute(c.runHistory.Context, nil, c.runHistory.Start, autods, c.Alert.UnjoinedOK, c.schedule.Search, c.schedule.Lookups, c.schedule.Conf.AlertSquelched(c.Alert))
	if err != nil {
		return nil, "", fmt.Errorf("%s: %v", v, err)
	}
	return res.Results, e.String(), nil
}

// Lookup returns the value for a key in the lookup table for the context's tagset.
func (c *Context) Lookup(table, key string) (string, error) {
	l, ok := c.schedule.Lookups[table]
	if !ok {
		return "", fmt.Errorf("unknown lookup table %v", table)
	}
	if v, ok := l.Get(key, c.Group); ok {
		return v, nil
	} else {
		return "", fmt.Errorf("no entry for key %v in table %v for tagset %v", key, table, c.Group)
	}
}

// Eval executes the given expression and returns a value with corresponding
// tags to the context's tags. If no such result is found, the first result with
// nil tags is returned. If no such result is found, nil is returned.
func (c *Context) Eval(v interface{}) (interface{}, error) {
	res, _, err := c.eval(v, true, false, 0)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no results returned")
	}
	// TODO: don't choose a random result, make sure there's exactly 1
	return res[0].Value, nil
}

// EvalAll returns the executed expression.
func (c *Context) EvalAll(v interface{}) (interface{}, error) {
	res, _, err := c.eval(v, false, false, 0)
	return res, err
}

func (c *Context) IsEmail() bool {
	return c.Attachments != nil
}

func (c *Context) graph(v interface{}, filter bool) (interface{}, error) {
	res, title, err := c.eval(v, filter, true, 1000)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	const width = 800
	const height = 600
	if c.IsEmail() {
		err := c.schedule.ExprPNG(nil, &buf, width, height, res, title, time.Now().UTC())
		if err != nil {
			return nil, err
		}
		name := fmt.Sprintf("%d.png", len(c.Attachments)+1)
		c.Attachments = append(c.Attachments, &conf.Attachment{
			Data:        buf.Bytes(),
			Filename:    name,
			ContentType: "image/png",
		})
		return template.HTML(fmt.Sprintf(`<img alt="%s" src="cid:%s" />`,
			template.HTMLEscapeString(fmt.Sprint(v)),
			name,
		)), nil
	}
	if err := c.schedule.ExprSVG(nil, &buf, width, height, res, title, time.Now().UTC()); err != nil {
		return nil, err
	}
	return template.HTML(buf.String()), nil
}

func (c *Context) Graph(v interface{}) (interface{}, error) {
	return c.graph(v, true)
}

func (c *Context) GraphAll(v interface{}) (interface{}, error) {
	return c.graph(v, false)
}

func (c *Context) GetMeta(metric, name string, v interface{}) (interface{}, error) {
	var t opentsdb.TagSet
	switch v := v.(type) {
	case string:
		var err error
		t, err = opentsdb.ParseTags(v)
		if err != nil {
			return t, err
		}
	case opentsdb.TagSet:
		t = v
	}
	meta := c.schedule.GetMetadata(metric, t)
	if name == "" {
		return meta, nil
	}
	for _, m := range meta {
		if m.Name == name {
			return m.Value, nil
		}
	}
	return nil, nil
}

func (c *Context) LeftJoin(q ...interface{}) (interface{}, error) {
	if len(q) < 2 {
		return nil, fmt.Errorf("need at least two expressions, got %v", len(q))
	}
	matrix := make([][]*expr.Result, 0)
	results := make([][]*expr.Result, len(q))
	for col, v := range q {
		res, _, err := c.eval(v, false, false, 0)
		if err != nil {
			return nil, err
		}
		results[col] = res
	}
	for row, first := range results[0] {
		matrix = append(matrix, make([]*expr.Result, len(q)))
		matrix[row][0] = first
		for col, res := range results[1:] {
			for _, r := range res {
				if first.Group.Subset(r.Group) {
					matrix[row][col+1] = r
					break
				}
				// Fill emtpy cells with NaN Value, so calling .Valie is not a nil pointer dereference
				matrix[row][col+1] = &expr.Result{Value: expr.Number(math.NaN())}
			}
		}
	}
	return matrix, nil
}
