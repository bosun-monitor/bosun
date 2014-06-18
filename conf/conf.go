package conf

import (
	"fmt"
	htemplate "html/template"
	"io/ioutil"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	ttemplate "text/template"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/bosun/conf/parse"
	"github.com/StackExchange/bosun/expr"
	eparse "github.com/StackExchange/bosun/expr/parse"
)

type Conf struct {
	Vars
	Name            string        // Config file name
	CheckFrequency  time.Duration // Time between alert checks: 5m
	WebDir          string        // Static content web directory: web
	TsdbHost        string        // OpenTSDB relay and query destination: ny-devtsdb04:4242
	RelayListen     string        // OpenTSDB relay listen address: :4242
	HttpListen      string        // Web server listen address: :80
	SmtpHost        string        // SMTP address: ny-mail:25
	EmailFrom       string
	StateFile       string
	TimeAndDate     []int // timeanddate.com cities list
	ResponseLimit   int64
	Unknown         time.Duration
	UnknownTemplate *Template
	Templates       map[string]*Template
	Alerts          map[string]*Alert
	Notifications   map[string]*Notification `json:"-"`
	RawText         string
	Macros          map[string]*Macro

	tree            *parse.Tree
	node            parse.Node
	unknownTemplate string
	bodies          *htemplate.Template
	subjects        *ttemplate.Template
}

// at marks the state to be on node n, for error reporting.
func (c *Conf) at(node parse.Node) {
	c.node = node
}

func (c *Conf) error(err error) {
	c.errorf(err.Error())
}

// errorf formats the error and terminates processing.
func (c *Conf) errorf(format string, args ...interface{}) {
	if c.node == nil {
		format = fmt.Sprintf("conf: %s: %s", c.Name, format)
	} else {
		location, context := c.tree.ErrorContext(c.node)
		format = fmt.Sprintf("conf: %s: at <%s>: %s", location, context, format)
	}
	panic(fmt.Errorf(format, args...))
}

// errRecover is the handler that turns panics into returns from the top
// level of Parse.
func errRecover(errp *error) {
	e := recover()
	if e != nil {
		switch err := e.(type) {
		case runtime.Error:
			panic(e)
		case error:
			*errp = err
		default:
			panic(e)
		}
	}
}

type Macro struct {
	Def string
	Vars
	Name   string
	Macros []string
}

type Alert struct {
	Def string
	Vars
	*Template        `json:"-"`
	Name             string
	Crit             *expr.Expr                `json:",omitempty"`
	Warn             *expr.Expr                `json:",omitempty"`
	Squelch          map[string]*regexp.Regexp `json:"-"`
	CritNotification map[string]*Notification  `json:"-"`
	WarnNotification map[string]*Notification  `json:"-"`
	Unknown          time.Duration
	Macros           []string

	crit, warn       string
	template         string
	squelch          string
	critNotification string
	warnNotification string
}

type Template struct {
	Def string
	Vars
	Name    string
	Body    *htemplate.Template `json:"-"`
	Subject *ttemplate.Template `json:"-"`

	body, subject string
}

type Notification struct {
	Def string
	Vars
	Name      string
	Email     []*mail.Address
	Post, Get *url.URL
	Print     bool
	Next      *Notification
	Timeout   time.Duration

	next      string
	email     string
	post, get string
}

func (n *Notification) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("conf: cannot json marshal notifications")
}

func (a *Alert) Squelched(tags opentsdb.TagSet) bool {
	if a.Squelch == nil {
		return false
	}
	for k, v := range a.Squelch {
		tagv, ok := tags[k]
		if !ok || !v.MatchString(tagv) {
			return false
		}
	}
	return true
}

type Vars map[string]string

func ParseFile(fname string) (*Conf, error) {
	f, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	return New(fname, string(f))
}

func New(name, text string) (c *Conf, err error) {
	defer errRecover(&err)
	c = &Conf{
		Name:           name,
		CheckFrequency: time.Minute * 5,
		HttpListen:     ":8070",
		RelayListen:    ":4242",
		WebDir:         "web",
		StateFile:      "bosun.state",
		ResponseLimit:  1 << 20, // 1MB
		Vars:           make(map[string]string),
		Templates:      make(map[string]*Template),
		Alerts:         make(map[string]*Alert),
		Notifications:  make(map[string]*Notification),
		RawText:        text,
		bodies:         htemplate.New(name).Funcs(htemplate.FuncMap(defaultFuncs)),
		subjects:       ttemplate.New(name).Funcs(defaultFuncs),
		Macros:         make(map[string]*Macro),
	}
	c.tree, err = parse.Parse(name, text)
	if err != nil {
		c.error(err)
	}
	saw := make(map[string]bool)
	for _, n := range c.tree.Root.Nodes {
		c.at(n)
		switch n := n.(type) {
		case *parse.PairNode:
			c.seen(n.Key.Text, saw)
			c.loadGlobal(n)
		case *parse.SectionNode:
			c.loadSection(n)
		default:
			c.errorf("unexpected parse node %s", n)
		}
	}
	if c.TsdbHost == "" {
		c.at(nil)
		c.errorf("tsdbHost required")
	}
	return
}

func (c *Conf) loadGlobal(p *parse.PairNode) {
	v := c.Expand(p.Val.Text, nil, false)
	switch k := p.Key.Text; k {
	case "checkFrequency":
		d, err := time.ParseDuration(v)
		if err != nil {
			c.error(err)
		}
		if d < time.Second {
			c.errorf("checkFrequency duration must be at least 1s")
		}
		c.CheckFrequency = d
	case "tsdbHost":
		c.TsdbHost = v
	case "httpListen":
		c.HttpListen = v
	case "relayListen":
		c.RelayListen = v
	case "webDir":
		c.WebDir = v
	case "smtpHost":
		c.SmtpHost = v
	case "emailFrom":
		c.EmailFrom = v
	case "stateFile":
		c.StateFile = v
	case "timeAndDate":
		sp := strings.Split(v, ",")
		var t []int
		for _, s := range sp {
			i, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil {
				c.error(err)
			}
			t = append(t, i)
		}
		c.TimeAndDate = t
	case "responseLimit":
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			c.error(err)
		}
		if i <= 0 {
			c.errorf("responseLimit must be > 0")
		}
		c.ResponseLimit = i
	case "unknown":
		d, err := time.ParseDuration(v)
		if err != nil {
			c.error(err)
		}
		if d < time.Second {
			c.errorf("unknown duration must be at least 1s")
		}
		c.Unknown = d
	case "unknownTemplate":
		c.unknownTemplate = v
		t, ok := c.Templates[c.unknownTemplate]
		if !ok {
			c.errorf("template not found: %s", c.unknownTemplate)
		}
		c.UnknownTemplate = t
	default:
		if !strings.HasPrefix(k, "$") {
			c.errorf("unknown key %s", k)
		}
		c.Vars[k] = v
		c.Vars[k[1:]] = c.Vars[k]
	}
}

func (c *Conf) loadSection(s *parse.SectionNode) {
	switch s.SectionType.Text {
	case "template":
		c.loadTemplate(s)
	case "alert":
		c.loadAlert(s)
	case "notification":
		c.loadNotification(s)
	case "macro":
		c.loadMacro(s)
	default:
		c.errorf("unknown section type: %s", s.SectionType.Text)
	}
}

type nodePair struct {
	node parse.Node
	key  string
	val  string
}

type sectionType int

const (
	sNormal sectionType = iota
	sMacro
)

func (c *Conf) getPairs(s *parse.SectionNode, vars Vars, st sectionType, used *[]string) []nodePair {
	saw := make(map[string]bool)
	var pairs []nodePair
	ignoreBadExpand := st == sMacro
	add := func(n parse.Node, k, v string) {
		c.seen(k, saw)
		if strings.HasPrefix(k, "$") {
			vars[k] = v
			if st != sMacro {
				vars[k[1:]] = v
			}
		} else {
			pairs = append(pairs, nodePair{
				node: n,
				key:  k,
				val:  v,
			})
		}
	}
	for _, n := range s.Nodes {
		c.at(n)
		v := c.Expand(n.Val.Text, vars, ignoreBadExpand)
		switch k := n.Key.Text; k {
		case "macro":
			m, ok := c.Macros[v]
			if !ok {
				c.errorf("macro not found: %s", v)
			}
			if used != nil {
				*used = append(*used, v)
			}
			for k, v := range m.Vars {
				add(n, k, c.Expand(v, vars, ignoreBadExpand))
			}
		default:
			add(n, k, v)
		}
	}
	return pairs
}

func (c *Conf) loadMacro(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Macros[name]; ok {
		c.errorf("duplicate macro name: %s", name)
	}
	m := Macro{
		Def:    s.RawText,
		Vars:   make(map[string]string),
		Name:   name,
		Macros: make([]string, 0),
	}
	for _, p := range c.getPairs(s, m.Vars, sMacro, &m.Macros) {
		m.Vars[p.key] = p.val
	}
	c.at(s)
	c.Macros[name] = &m
}

var defaultFuncs = ttemplate.FuncMap{
	"bytes": func(v string) ByteSize {
		f, _ := strconv.ParseFloat(v, 64)
		return ByteSize(f)
	},
	"short": func(v string) string {
		return strings.SplitN(v, ".", 2)[0]
	},
	"replace": strings.Replace,
}

func (c *Conf) loadTemplate(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Templates[name]; ok {
		c.errorf("duplicate template name: %s", name)
	}
	t := Template{
		Def:  s.RawText,
		Vars: make(map[string]string),
		Name: name,
	}
	funcs := ttemplate.FuncMap{
		"V": func(v string) string {
			return c.Expand(v, t.Vars, false)
		},
	}
	saw := make(map[string]bool)
	for _, p := range s.Nodes {
		c.at(p)
		c.seen(p.Key.Text, saw)
		v := p.Val.Text
		switch k := p.Key.Text; k {
		case "body":
			t.body = v
			tmpl := c.bodies.New(name).Funcs(htemplate.FuncMap(funcs))
			_, err := tmpl.Parse(t.body)
			if err != nil {
				c.error(err)
			}
			t.Body = tmpl
		case "subject":
			t.subject = v
			tmpl := c.subjects.New(name).Funcs(funcs)
			_, err := tmpl.Parse(t.subject)
			if err != nil {
				c.error(err)
			}
			t.Subject = tmpl
		default:
			if !strings.HasPrefix(k, "$") {
				c.errorf("unknown key %s", k)
			}
			t.Vars[k] = v
			t.Vars[k[1:]] = t.Vars[k]
		}
	}
	c.at(s)
	if t.Body == nil && t.Subject == nil {
		c.errorf("neither body or subject specified")
	}
	c.Templates[name] = &t
}

func (c *Conf) loadAlert(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Alerts[name]; ok {
		c.errorf("duplicate alert name: %s", name)
	}
	a := Alert{
		Def:    s.RawText,
		Vars:   make(map[string]string),
		Name:   name,
		Macros: make([]string, 0),
	}
	for _, p := range c.getPairs(s, a.Vars, sNormal, &a.Macros) {
		c.at(p.node)
		v := p.val
		switch p.key {
		case "template":
			a.template = v
			t, ok := c.Templates[a.template]
			if !ok {
				c.errorf("template not found %s", a.template)
			}
			a.Template = t
		case "crit":
			a.crit = v
			crit, err := expr.New(a.crit)
			if err != nil {
				c.error(err)
			}
			switch crit.Root.Return() {
			case eparse.TYPE_NUMBER, eparse.TYPE_SCALAR:
				// break
			default:
				c.errorf("crit must return a number")
			}
			a.Crit = crit
		case "warn":
			a.warn = v
			warn, err := expr.New(a.warn)
			if err != nil {
				c.error(err)
			}
			switch warn.Root.Return() {
			case eparse.TYPE_NUMBER, eparse.TYPE_SCALAR:
				// break
			default:
				c.errorf("warn must return a number")
			}
			a.Warn = warn
		case "squelch":
			a.squelch = v
			squelch, err := opentsdb.ParseTags(a.squelch)
			if err != nil {
				c.error(err)
			}
			a.Squelch = make(map[string]*regexp.Regexp)
			for k, v := range squelch {
				re, err := regexp.Compile(v)
				if err != nil {
					c.error(err)
				}
				a.Squelch[k] = re
			}
		case "critNotification":
			a.critNotification = v
			a.CritNotification = make(map[string]*Notification)
			for _, s := range strings.Split(a.critNotification, ",") {
				s = strings.TrimSpace(s)
				n, ok := c.Notifications[s]
				if !ok {
					c.errorf("unknown notification %s", s)
				}
				a.CritNotification[s] = n
			}
		case "warnNotification":
			a.warnNotification = v
			a.WarnNotification = make(map[string]*Notification)
			for _, s := range strings.Split(a.warnNotification, ",") {
				s = strings.TrimSpace(s)
				n, ok := c.Notifications[s]
				if !ok {
					c.errorf("unknown notification %s", s)
				}
				a.WarnNotification[s] = n
			}
		case "unknown":
			d, err := time.ParseDuration(v)
			if err != nil {
				c.error(err)
			}
			if d < time.Second {
				c.errorf("unknown duration must be at least 1s")
			}
			a.Unknown = d
		default:
			c.errorf("unknown key %s", p.key)
		}
	}
	c.at(s)
	if a.Crit == nil && a.Warn == nil {
		c.errorf("neither crit or warn specified")
	}
	c.Alerts[name] = &a
}

func (c *Conf) loadNotification(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Notifications[name]; ok {
		c.errorf("duplicate notification name: %s", name)
	}
	n := Notification{
		Def:  s.RawText,
		Vars: make(map[string]string),
		Name: name,
	}
	c.Notifications[name] = &n
	for _, p := range c.getPairs(s, n.Vars, sNormal, nil) {
		c.at(p.node)
		v := p.val
		switch k := p.key; k {
		case "email":
			if c.SmtpHost == "" || c.EmailFrom == "" {
				c.errorf("email notifications require both smtpHost and emailFrom to be set")
			}
			n.email = v
			email, err := mail.ParseAddressList(n.email)
			if err != nil {
				c.error(err)
			}
			n.Email = email
		case "post":
			n.post = v
			post, err := url.Parse(n.post)
			if err != nil {
				c.error(err)
			}
			n.Post = post
		case "get":
			n.get = v
			get, err := url.Parse(n.get)
			if err != nil {
				c.error(err)
			}
			n.Get = get
		case "print":
			n.Print = true
		case "next":
			n.next = v
			next, ok := c.Notifications[n.next]
			if !ok {
				c.errorf("unknown notification %s", n.next)
			}
			n.Next = next
		case "timeout":
			d, err := time.ParseDuration(v)
			if err != nil {
				c.error(err)
			}
			n.Timeout = d
		default:
			c.errorf("unknown key %s", k)
		}
	}
	c.at(s)
	if n.Timeout > 0 && n.Next == nil {
		c.errorf("timeout specified without next")
	}
}

var exRE = regexp.MustCompile(`\$(?:\w+|\{\w+\})`)

func (c *Conf) Expand(v string, vars map[string]string, ignoreBadExpand bool) string {
	return exRE.ReplaceAllStringFunc(v, func(s string) string {
		var n string
		if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
			n = os.Getenv(s[2 : len(s)-1])
		}
		if _n, ok := c.Vars[s]; ok {
			n = _n
		}
		if _n, ok := vars[s]; ok {
			n = _n
		}
		if n == "" {
			if ignoreBadExpand {
				return s
			}
			c.errorf("unknown variable %s", s)
		}
		return c.Expand(n, vars, ignoreBadExpand)
	})
}

func (c *Conf) seen(v string, m map[string]bool) {
	if m[v] {
		c.errorf("duplicate key: %s", v)
	}
	m[v] = true
}
