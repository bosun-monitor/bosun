package sched

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"os"
	"strings"
	"text/template/parse"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/bosun/conf"
	"github.com/StackExchange/bosun/expr"
	eparse "github.com/StackExchange/bosun/expr/parse"
)

type Context struct {
	*State
	Alert *conf.Alert

	schedule    *Schedule
	Attachments []*conf.Attachment
}

func (s *Schedule) Data(st *State, a *conf.Alert, isEmail bool) *Context {
	c := Context{
		State:    st,
		Alert:    a,
		schedule: s,
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

func (c *Context) Expr(v string) (string, error) {
	q := "expr=" + base64.URLEncoding.EncodeToString([]byte(opentsdb.ReplaceTags(v, c.Group)))
	u := url.URL{
		Scheme:   "http",
		Host:     c.schedule.Conf.HttpListen,
		Path:     "/expr",
		RawQuery: q,
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

func (c *Context) Rule() (string, error) {
	t, err := c.schedule.MakeTemplates()
	if err != nil {
		return "", err
	}
	adef := base64.URLEncoding.EncodeToString([]byte(t.Alerts[c.Alert.Name]))
	for i, rune := range t.Alerts[c.Alert.Name] {
		if int(rune) > 127 {
			fmt.Printf("%d: %c %i\n", i, rune, int(rune))
		}
	}
	tdef := base64.URLEncoding.EncodeToString([]byte(t.Templates[c.Alert.Template.Name]))
	q := "alert=" + adef + "&template=" + tdef
	u := url.URL{
		Scheme:   "http",
		Host:     c.schedule.Conf.HttpListen,
		Path:     "/rule",
		RawQuery: q,
	}
	if strings.HasPrefix(c.schedule.Conf.HttpListen, ":") {
		h, err := os.Hostname()
		if err != nil {
			return "", err
		}
		u.Host = h + u.Host
	}
	return u.RequestURI(), nil
}

func (s *Schedule) ExecuteBody(w io.Writer, a *conf.Alert, st *State, isEmail bool) ([]*conf.Attachment, error) {
	t := a.Template
	if t == nil || t.Body == nil {
		return nil, nil
	}
	c := s.Data(st, a, isEmail)
	return c.Attachments, t.Body.Execute(w, c)
}

func (s *Schedule) ExecuteSubject(w io.Writer, a *conf.Alert, st *State) error {
	t := a.Template
	if t == nil || t.Subject == nil {
		return nil
	}
	return t.Subject.Execute(w, s.Data(st, a, false))
}

func (c *Context) eval(v interface{}, filter bool, series bool, autods int) ([]*expr.Result, error) {
	var e *expr.Expr
	var err error
	switch v := v.(type) {
	case string:
		e, err = expr.New(v)
	case *expr.Expr:
		e = v
	default:
		return nil, fmt.Errorf("expected string or expression, got %T (%v)", v, v)
	}
	if err != nil {
		return nil, fmt.Errorf("%v: %v", v, err)
	}
	var results []*expr.Result
	if series && e.Root.Return() != eparse.TYPE_SERIES {
		return results, fmt.Errorf("egraph: requires an expression that returns a series")
	}
	res, _, err := e.Execute(c.schedule.cache, nil, c.schedule.CheckStart, autods, c.Alert.UnjoinedOK, c.schedule.Search, c.schedule.Lookups, c.schedule.Conf.AlertSquelched(c.Alert))
	if err != nil {
		return results, fmt.Errorf("%s: %v", v, err)
	}
	if !filter {
		return res.Results, nil
	}
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
	return nil, nil
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
	res, err := c.eval(v, true, false, 0)
	if err != nil {
		return nil, err
	}
	if len(res) != 1 {
		return nil, fmt.Errorf("expected 1 result, got %v", len(res))
	}
	return res[0].Value, nil
}

// EvalAll returns the executed expression.
func (c *Context) EvalAll(v interface{}) (interface{}, error) {
	return c.eval(v, false, false, 0)
}

func (c *Context) IsEmail() bool {
	return c.Attachments != nil
}

func (c *Context) graph(v interface{}, filter bool) (interface{}, error) {
	res, err := c.eval(v, filter, true, 1000)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := c.schedule.ExprGraph(nil, &buf, res, fmt.Sprint(v), time.Now().UTC()); err != nil {
		return nil, err
	}
	if c.IsEmail() {
		name := fmt.Sprintf("%d.svg", len(c.Attachments)+1)
		c.Attachments = append(c.Attachments, &conf.Attachment{
			Data:        buf.Bytes(),
			Filename:    name,
			ContentType: "image/svg+xml",
		})
		return template.HTML(fmt.Sprintf(`<img alt="%s" src="cid:%s" />`,
			template.HTMLEscapeString(fmt.Sprint(v)),
			name,
		)), nil
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
	fm := make([]metadata.Metasend, 0)
	for _, m := range meta {
		if m.Name == name {
			fm = append(fm, m)
		}
	}
	return fm, nil
}

func (c *Context) LeftJoin(q ...interface{}) (interface{}, error) {
	if len(q) < 2 {
		return nil, fmt.Errorf("need at least two expressions, got %v", len(q))
	}
	matrix := make([][]*expr.Result, 0)
	results := make([][]*expr.Result, len(q))
	for col, v := range q {
		res, err := c.eval(v, false, false, 0)
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
				}
			}
		}
	}
	return matrix, nil
}

var builtins = template.FuncMap{
	"and":      nilFunc,
	"call":     nilFunc,
	"html":     nilFunc,
	"index":    nilFunc,
	"js":       nilFunc,
	"len":      nilFunc,
	"not":      nilFunc,
	"or":       nilFunc,
	"print":    nilFunc,
	"printf":   nilFunc,
	"println":  nilFunc,
	"urlquery": nilFunc,
	"eq":       nilFunc,
	"ge":       nilFunc,
	"gt":       nilFunc,
	"le":       nilFunc,
	"lt":       nilFunc,
	"ne":       nilFunc,

	// HTML-specific funcs
	"html_template_attrescaper":     nilFunc,
	"html_template_commentescaper":  nilFunc,
	"html_template_cssescaper":      nilFunc,
	"html_template_cssvaluefilter":  nilFunc,
	"html_template_htmlnamefilter":  nilFunc,
	"html_template_htmlescaper":     nilFunc,
	"html_template_jsregexpescaper": nilFunc,
	"html_template_jsstrescaper":    nilFunc,
	"html_template_jsvalescaper":    nilFunc,
	"html_template_nospaceescaper":  nilFunc,
	"html_template_rcdataescaper":   nilFunc,
	"html_template_urlescaper":      nilFunc,
	"html_template_urlfilter":       nilFunc,
	"html_template_urlnormalizer":   nilFunc,

	// bosun-specific funcs
	"V":       nilFunc,
	"bytes":   nilFunc,
	"replace": nilFunc,
	"short":   nilFunc,
}

func nilFunc() {}

type TA struct {
	Templates map[string]string
	Alerts    map[string]string
}

func (schedule *Schedule) MakeTemplates() (*TA, error) {
	templates := make(map[string]string)
	for name, template := range schedule.Conf.Templates {
		incl := map[string]bool{name: true}
		var parseSection func(*conf.Template) error
		parseTemplate := func(s string) error {
			trees, err := parse.Parse("", s, "", "", builtins)
			if err != nil {
				return err
			}
			for _, node := range trees[""].Root.Nodes {
				switch node := node.(type) {
				case *parse.TemplateNode:
					if incl[node.Name] {
						continue
					}
					incl[node.Name] = true
					if err := parseSection(schedule.Conf.Templates[node.Name]); err != nil {
						return err
					}
				}
			}
			return nil
		}
		parseSection = func(s *conf.Template) error {
			if s.Body != nil {
				if err := parseTemplate(s.Body.Tree.Root.String()); err != nil {
					return err
				}
			}
			if s.Subject != nil {
				if err := parseTemplate(s.Subject.Tree.Root.String()); err != nil {
					return err
				}
			}
			return nil
		}
		if err := parseSection(template); err != nil {
			return nil, err
		}
		delete(incl, name)
		templates[name] = template.Def
		for n := range incl {
			t := schedule.Conf.Templates[n]
			if t == nil {
				continue
			}
			templates[name] += "\n\n" + t.Def
		}
	}
	alerts := make(map[string]string)
	for name, alert := range schedule.Conf.Alerts {
		var add func([]string)
		add = func(macros []string) {
			for _, macro := range macros {
				m := schedule.Conf.Macros[macro]
				add(m.Macros)
				alerts[name] += m.Def + "\n\n"
			}
		}
		lookups := make(map[string]bool)
		walk := func(n eparse.Node) {
			eparse.Walk(n, func(n eparse.Node) {
				switch n := n.(type) {
				case *eparse.FuncNode:
					if n.Name != "lookup" || len(n.Args) == 0 {
						return
					}
					switch n := n.Args[0].(type) {
					case *eparse.StringNode:
						if lookups[n.Text] {
							return
						}
						lookups[n.Text] = true
						l := schedule.Conf.Lookups[n.Text]
						if l == nil {
							return
						}
						alerts[name] += l.Def + "\n\n"
					}
				}
			})
		}
		walkNotifications := func(n *conf.Notifications) {
			for _, v := range n.Lookups {
				if lookups[v.Name] {
					return
				}
				lookups[v.Name] = true
				alerts[name] += v.Def + "\n\n"
			}
		}
		if alert.CritNotification != nil {
			walkNotifications(alert.CritNotification)
		}
		if alert.WarnNotification != nil {
			walkNotifications(alert.WarnNotification)
		}
		add(alert.Macros)
		if alert.Crit != nil {
			walk(alert.Crit.Tree.Root)
		}
		if alert.Warn != nil {
			walk(alert.Warn.Tree.Root)
		}
		alerts[name] += alert.Def
	}
	return &TA{
		templates,
		alerts,
	}, nil
}
