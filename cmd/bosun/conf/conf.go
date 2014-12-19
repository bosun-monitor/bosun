package conf // import "bosun.org/cmd/bosun/conf"

import (
	"encoding/json"
	"fmt"
	htemplate "html/template"
	"io/ioutil"
	"log"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	ttemplate "text/template"
	tparse "text/template/parse"
	"time"

	"bosun.org/_third_party/github.com/MiniProfiler/go/miniprofiler"

	"bosun.org/cmd/bosun/conf/parse"
	"bosun.org/cmd/bosun/expr"
	eparse "bosun.org/cmd/bosun/expr/parse"
	"bosun.org/opentsdb"
)

type Conf struct {
	Vars
	Name            string        // Config file name
	CheckFrequency  time.Duration // Time between alert checks: 5m
	TSDBHost        string        // OpenTSDB relay and query destination: ny-devtsdb04:4242
	GraphiteHost    string        // host for graphite, like http://foo.bar.baz
	HTTPListen      string        // Web server listen address: :80
	Hostname        string
	RelayListen     string // OpenTSDB relay listen address: :4242
	SMTPHost        string // SMTP address: ny-mail:25
	SMTPUsername    string // SMTP username
	SMTPPassword    string // SMTP password
	Ping            bool
	EmailFrom       string
	StateFile       string
	TimeAndDate     []int // timeanddate.com cities list
	ResponseLimit   int64
	UnknownTemplate *Template
	Templates       map[string]*Template
	Alerts          map[string]*Alert
	Notifications   map[string]*Notification `json:"-"`
	RawText         string
	Macros          map[string]*Macro
	Lookups         map[string]*Lookup
	Squelch         Squelches `json:"-"`
	Quiet           bool

	tree            *parse.Tree
	node            parse.Node
	unknownTemplate string
	bodies          *htemplate.Template
	subjects        *ttemplate.Template
	squelch         []string
}

type Squelch map[string]*regexp.Regexp

type Squelches struct {
	s []Squelch
}

func (s *Squelches) Add(v string) error {
	tags, err := opentsdb.ParseTags(v)
	if tags == nil && err != nil {
		return err
	}
	sq := make(Squelch)
	for k, v := range tags {
		re, err := regexp.Compile(v)
		if err != nil {
			return err
		}
		sq[k] = re
	}
	s.s = append(s.s, sq)
	return nil
}

func (s *Squelches) Squelched(tags opentsdb.TagSet) bool {
	for _, q := range s.s {
		if q.Squelched(tags) {
			return true
		}
	}
	return false
}

func (s Squelch) Squelched(tags opentsdb.TagSet) bool {
	if len(s) == 0 {
		return false
	}
	for k, v := range s {
		tagv, ok := tags[k]
		if !ok || !v.MatchString(tagv) {
			return false
		}
	}
	return true
}

func (c *Conf) AlertSquelched(a *Alert) func(opentsdb.TagSet) bool {
	return func(tags opentsdb.TagSet) bool {
		return c.Squelched(a, tags)
	}
}

func (c *Conf) Squelched(a *Alert, tags opentsdb.TagSet) bool {
	return c.Squelch.Squelched(tags) || a.Squelch.Squelched(tags)
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

type Lookup struct {
	Def     string
	Name    string
	Tags    []string
	Entries []*Entry
}

func (lookup *Lookup) ToExpr() *ExprLookup {
	l := ExprLookup{
		Tags: lookup.Tags,
	}
	for _, entry := range lookup.Entries {
		l.Entries = append(l.Entries, entry.ExprEntry)
	}
	return &l
}

type Entry struct {
	*ExprEntry
	Def  string
	Name string
}

type Macro struct {
	Def    string
	Pairs  []nodePair
	Name   string
	Macros []string
}

type Alert struct {
	Def string
	Vars
	*Template        `json:"-"`
	Name             string
	Crit             *expr.Expr `json:",omitempty"`
	Warn             *expr.Expr `json:",omitempty"`
	Squelch          Squelches  `json:"-"`
	CritNotification *Notifications
	WarnNotification *Notifications
	Unknown          time.Duration
	IgnoreUnknown    bool
	Macros           []string `json:"-"`
	UnjoinedOK       bool     `json:",omitempty"`
	returnType       eparse.FuncType

	crit, warn string
	template   string
	squelch    []string
	tags       eparse.Tags
}

type Notifications struct {
	Notifications map[string]*Notification `json:"-"`
	// Table key -> table
	Lookups map[string]*Lookup
}

// Get returns the set of notifications based on given tags.
func (ns *Notifications) Get(c *Conf, tags opentsdb.TagSet) map[string]*Notification {
	nots := make(map[string]*Notification)
	for name, n := range ns.Notifications {
		nots[name] = n
	}
	for key, lookup := range ns.Lookups {
		l := lookup.ToExpr()
		val, ok := l.Get(key, tags)
		if !ok {
			continue
		}
		ns, err := c.parseNotifications(val)
		if err != nil {
			// Should already be checked by conf parser.
			panic(err)
		}
		for name, n := range ns {
			nots[name] = n
		}
	}
	return nots
}

// parseNotifications parses the comma-separated string v for notifications and
// returns them.
func (c *Conf) parseNotifications(v string) (map[string]*Notification, error) {
	ns := make(map[string]*Notification)
	for _, s := range strings.Split(v, ",") {
		s = strings.TrimSpace(s)
		n := c.Notifications[s]
		if n == nil {
			return nil, fmt.Errorf("unknown notification %s", s)
		}
		ns[s] = n
	}
	return ns, nil
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
	Body      *ttemplate.Template
	Print     bool
	Next      *Notification
	Timeout   time.Duration

	next      string
	email     string
	post, get string
	body      string
}

func (n *Notification) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("conf: cannot json marshal notifications")
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
		HTTPListen:     ":8070",
		StateFile:      "bosun.state",
		ResponseLimit:  1 << 20, // 1MB
		Vars:           make(map[string]string),
		Templates:      make(map[string]*Template),
		Alerts:         make(map[string]*Alert),
		Notifications:  make(map[string]*Notification),
		RawText:        text,
		bodies:         htemplate.New(name).Funcs(htemplate.FuncMap(defaultFuncs)),
		subjects:       ttemplate.New(name).Funcs(defaultFuncs),
		Lookups:        make(map[string]*Lookup),
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
	if c.TSDBHost == "" {
		c.at(nil)
		c.errorf("tsdbHost required")
	}
	if c.Hostname == "" {
		c.Hostname = c.HTTPListen
		if strings.HasPrefix(c.Hostname, ":") {
			h, err := os.Hostname()
			if err != nil {
				c.at(nil)
				c.error(err)
			}
			c.Hostname = h + c.Hostname
		}
	}
	return
}

func (c *Conf) loadGlobal(p *parse.PairNode) {
	v := c.Expand(p.Val.Text, nil, false)
	switch k := p.Key.Text; k {
	case "checkFrequency":
		od, err := opentsdb.ParseDuration(v)
		if err != nil {
			c.error(err)
		}
		d := time.Duration(od)
		if d < time.Second {
			c.errorf("checkFrequency duration must be at least 1s")
		}
		c.CheckFrequency = d
	case "tsdbHost":
		if !strings.Contains(v, ":") {
			v += ":4242"
		}
		c.TSDBHost = v
	case "graphiteHost":
		c.GraphiteHost = v
	case "httpListen":
		c.HTTPListen = v
	case "hostname":
		c.Hostname = v
	case "relayListen":
		c.RelayListen = v
	case "smtpHost":
		c.SMTPHost = v
	case "smtpUsername":
		c.SMTPUsername = v
	case "smtpPassword":
		c.SMTPPassword = v
	case "emailFrom":
		c.EmailFrom = v
	case "stateFile":
		c.StateFile = v
	case "ping":
		c.Ping = true
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
	case "unknownTemplate":
		c.unknownTemplate = v
		t, ok := c.Templates[c.unknownTemplate]
		if !ok {
			c.errorf("template not found: %s", c.unknownTemplate)
		}
		c.UnknownTemplate = t
	case "squelch":
		c.squelch = append(c.squelch, v)
		if err := c.Squelch.Add(v); err != nil {
			c.error(err)
		}
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
	case "lookup":
		c.loadLookup(s)
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
		if vars != nil && strings.HasPrefix(k, "$") {
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
	for _, n := range s.Nodes.Nodes {
		c.at(n)
		switch n := n.(type) {
		case *parse.PairNode:
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
				for _, p := range m.Pairs {
					add(p.node, p.key, c.Expand(p.val, vars, ignoreBadExpand))
				}
			default:
				add(n, k, v)
			}
		default:
			c.errorf("unexpected node")
		}
	}
	return pairs
}

func (c *Conf) loadLookup(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Lookups[name]; ok {
		c.errorf("duplicate lookup name: %s", name)
	}
	l := Lookup{
		Def:  s.RawText,
		Name: name,
	}
	var lookupTags opentsdb.TagSet
	saw := make(map[string]bool)
	for _, n := range s.Nodes.Nodes {
		c.at(n)
		switch n := n.(type) {
		case *parse.SectionNode:
			if n.SectionType.Text != "entry" {
				c.errorf("unexpected subsection type")
			}
			tags, err := opentsdb.ParseTags(n.Name.Text)
			if tags == nil && err != nil {
				c.error(err)
			}
			if _, ok := saw[tags.String()]; ok {
				c.errorf("duplicate entry")
			}
			saw[tags.String()] = true
			if len(tags) == 0 {
				c.errorf("lookup entries require tags")
			}
			empty := make(opentsdb.TagSet)
			for k := range tags {
				empty[k] = ""
			}
			if len(lookupTags) == 0 {
				lookupTags = empty
				for k := range empty {
					l.Tags = append(l.Tags, k)
				}
			} else if !lookupTags.Equal(empty) {
				c.errorf("lookup tags mismatch, expected %v", lookupTags)
			}
			e := Entry{
				Def:  n.RawText,
				Name: n.Name.Text,
				ExprEntry: &ExprEntry{
					AlertKey: expr.NewAlertKey("", tags),
					Values:   make(map[string]string),
				},
			}
			for _, en := range n.Nodes.Nodes {
				c.at(en)
				switch en := en.(type) {
				case *parse.PairNode:
					e.Values[en.Key.Text] = en.Val.Text
				default:
					c.errorf("unexpected node")
				}
			}
			l.Entries = append(l.Entries, &e)
		default:
			c.errorf("unexpected node")
		}
	}
	c.at(s)
	c.Lookups[name] = &l
}

func (c *Conf) loadMacro(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Macros[name]; ok {
		c.errorf("duplicate macro name: %s", name)
	}
	m := Macro{
		Def:    s.RawText,
		Name:   name,
		Macros: make([]string, 0),
	}
	for _, p := range c.getPairs(s, nil, sMacro, &m.Macros) {
		m.Pairs = append(m.Pairs, p)
	}
	c.at(s)
	c.Macros[name] = &m
}

var defaultFuncs = ttemplate.FuncMap{
	"bytes": func(v interface{}) (ByteSize, error) {
		switch v := v.(type) {
		case string:
			f, err := strconv.ParseFloat(v, 64)
			return ByteSize(f), err
		case int:
			return ByteSize(v), nil
		case float64:
			return ByteSize(v), nil
		case expr.Number:
			return ByteSize(v), nil
		case expr.Scalar:
			return ByteSize(v), nil
		}
		return ByteSize(0), fmt.Errorf("unexpected type: %T (%v)", v, v)
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
	for _, p := range s.Nodes.Nodes {
		c.at(p)
		switch p := p.(type) {
		case *parse.PairNode:
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
		default:
			c.errorf("unexpected node")
		}
	}
	c.at(s)
	if t.Body == nil && t.Subject == nil {
		c.errorf("neither body or subject specified")
	}
	c.Templates[name] = &t
}

var lookupNotificationRE = regexp.MustCompile(`^lookup\("(.*)", "(.*)"\)$`)

func (c *Conf) loadAlert(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Alerts[name]; ok {
		c.errorf("duplicate alert name: %s", name)
	}
	a := Alert{
		Def:              s.RawText,
		Vars:             make(map[string]string),
		Name:             name,
		Macros:           make([]string, 0),
		CritNotification: new(Notifications),
		WarnNotification: new(Notifications),
	}
	procNotification := func(v string, ns *Notifications) {
		if lookup := lookupNotificationRE.FindStringSubmatch(v); lookup != nil {
			if ns.Lookups == nil {
				ns.Lookups = make(map[string]*Lookup)
			}
			l := c.Lookups[lookup[1]]
			if l == nil {
				c.errorf("unknown lookup table %s", lookup[1])
			}
			for _, e := range l.Entries {
				for k, v := range e.Values {
					if k != lookup[2] {
						continue
					}
					if _, err := c.parseNotifications(v); err != nil {
						c.errorf("lookup %s: %v", v, err)
					}
				}
			}
			ns.Lookups[lookup[2]] = l
			return
		}
		n, err := c.parseNotifications(v)
		if err != nil {
			c.error(err)
		}
		if ns.Notifications == nil {
			ns.Notifications = make(map[string]*Notification)
		}
		for k, v := range n {
			ns.Notifications[k] = v
		}
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
			crit, err := c.NewExpr(a.crit)
			if err != nil {
				c.error(err)
			}
			switch crit.Root.Return() {
			case eparse.TypeNumber, eparse.TypeScalar:
				// break
			default:
				c.errorf("crit must return a number")
			}
			a.Crit = crit
		case "warn":
			a.warn = v
			warn, err := c.NewExpr(a.warn)
			if err != nil {
				c.error(err)
			}
			switch warn.Root.Return() {
			case eparse.TypeNumber, eparse.TypeScalar:
				// break
			default:
				c.errorf("warn must return a number")
			}
			a.Warn = warn
		case "squelch":
			a.squelch = append(a.squelch, v)
			if err := a.Squelch.Add(v); err != nil {
				c.error(err)
			}
		case "critNotification":
			procNotification(v, a.CritNotification)
		case "warnNotification":
			procNotification(v, a.WarnNotification)
		case "unknown":
			od, err := opentsdb.ParseDuration(v)
			if err != nil {
				c.error(err)
			}
			d := time.Duration(od)
			if d < time.Second {
				c.errorf("unknown duration must be at least 1s")
			}
			a.Unknown = d
		case "unjoinedOk":
			a.UnjoinedOK = true
		case "ignoreUnknown":
			a.IgnoreUnknown = true
		default:
			c.errorf("unknown key %s", p.key)
		}
	}
	c.at(s)
	if a.Crit == nil && a.Warn == nil {
		c.errorf("neither crit or warn specified")
	}
	var tags eparse.Tags
	var ret eparse.FuncType
	if a.Crit != nil {
		ctags, err := a.Crit.Root.Tags()
		if err != nil {
			c.error(err)
		}
		tags = ctags
		ret = a.Crit.Root.Return()
	}
	if a.Warn != nil {
		wtags, err := a.Warn.Root.Tags()
		if err != nil {
			c.error(err)
		}
		wret := a.Warn.Root.Return()
		if a.Crit == nil {
			tags = wtags
			ret = wret
		} else if ret != wret {
			c.errorf("crit and warn expressions must return same type (%v != %v)", ret, wret)
		} else if !tags.Equal(wtags) {
			c.errorf("crit tags (%v) and warn tags (%v) must be equal", tags, wtags)
		}
	}
	a.tags = tags
	a.returnType = ret
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
	funcs := ttemplate.FuncMap{
		"V": func(v string) string {
			return c.Expand(v, n.Vars, false)
		},
		"json": func(v interface{}) string {
			b, err := json.Marshal(v)
			if err != nil {
				log.Println(err)
			}
			return string(b)
		},
	}
	c.Notifications[name] = &n
	for _, p := range c.getPairs(s, n.Vars, sNormal, nil) {
		c.at(p.node)
		v := p.val
		switch k := p.key; k {
		case "email":
			if c.SMTPHost == "" || c.EmailFrom == "" {
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
			d, err := opentsdb.ParseDuration(v)
			if err != nil {
				c.error(err)
			}
			n.Timeout = time.Duration(d)
		case "body":
			n.body = v
			tmpl := ttemplate.New(name).Funcs(funcs)
			_, err := tmpl.Parse(n.body)
			if err != nil {
				c.error(err)
			}
			n.Body = tmpl
		default:
			c.errorf("unknown key %s", k)
		}
	}
	c.at(s)
	if n.Timeout > 0 && n.Next == nil {
		c.errorf("timeout specified without next")
	}
}

var exRE = regexp.MustCompile(`\$(?:[\w.]+|\{[\w.]+\})`)

func (c *Conf) Expand(v string, vars map[string]string, ignoreBadExpand bool) string {
	ss := exRE.ReplaceAllStringFunc(v, func(s string) string {
		var n string
		if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
			s = "$" + s[2:len(s)-1]
		}
		if strings.HasPrefix(s, "$env.") {
			n = os.Getenv(s[5:])
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
	return ss
}

func (c *Conf) seen(v string, m map[string]bool) {
	if m[v] {
		switch v {
		case "squelch", "critNotification", "warnNotification":
			// ignore
		default:
			c.errorf("duplicate key: %s", v)
		}
	}
	m[v] = true
}

var builtins = htemplate.FuncMap{
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

type AlertTemplateStrings struct {
	Templates   map[string]string
	Alerts      map[string]string
	Assocations map[string]string
}

func (c *Conf) AlertTemplateStrings() (*AlertTemplateStrings, error) {
	templates := make(map[string]string)
	for name, template := range c.Templates {
		incl := map[string]bool{name: true}
		var parseSection func(*Template) error
		parseTemplate := func(s string) error {
			trees, err := tparse.Parse("", s, "", "", builtins)
			if err != nil {
				return err
			}
			for _, node := range trees[""].Root.Nodes {
				switch node := node.(type) {
				case *tparse.TemplateNode:
					if incl[node.Name] {
						continue
					}
					incl[node.Name] = true
					if err := parseSection(c.Templates[node.Name]); err != nil {
						return err
					}
				}
			}
			return nil
		}
		parseSection = func(s *Template) error {
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
			t := c.Templates[n]
			if t == nil {
				continue
			}
			templates[name] += "\n\n" + t.Def
		}
	}
	alerts := make(map[string]string)
	t_associations := make(map[string]string)
	for name, alert := range c.Alerts {
		var add func([]string)
		add = func(macros []string) {
			for _, macro := range macros {
				m := c.Macros[macro]
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
						l := c.Lookups[n.Text]
						if l == nil {
							return
						}
						alerts[name] += l.Def + "\n\n"
					}
				}
			})
		}
		walkNotifications := func(n *Notifications) {
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
		if alert.Template != nil {
			t_associations[alert.Name] = alert.Template.Name
		}
	}
	return &AlertTemplateStrings{
		templates,
		alerts,
		t_associations,
	}, nil
}

func (c *Conf) NewExpr(s string) (*expr.Expr, error) {
	return expr.New(s, c.Funcs())
}

func (c *Conf) Funcs() map[string]eparse.Func {
	lookup := func(e *expr.State, T miniprofiler.Timer, lookup, key string) (results *expr.Results, err error) {
		results = new(expr.Results)
		results.IgnoreUnjoined = true
		l := c.Lookups[lookup]
		if l == nil {
			return nil, fmt.Errorf("lookup table not found: %v", lookup)
		}
		lookups := l.ToExpr()
		if lookups == nil {
			err = fmt.Errorf("lookup table not found: %v", lookup)
			return
		}
		var tags []opentsdb.TagSet
		for _, tag := range lookups.Tags {
			var next []opentsdb.TagSet
			for _, value := range e.Search.TagValuesByTagKey(tag) {
				for _, s := range tags {
					t := s.Copy()
					t[tag] = value
					next = append(next, t)
				}
				if len(tags) == 0 {
					next = append(next, opentsdb.TagSet{tag: value})
				}
			}
			tags = next
		}
		for _, tag := range tags {
			value, ok := lookups.Get(key, tag)
			if !ok {
				continue
			}
			var num float64
			num, err = strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, err
			}
			results.Results = append(results.Results, &expr.Result{
				Value: expr.Number(num),
				Group: tag,
			})
		}
		return results, nil
	}
	tagLookup := func(args []eparse.Node) (eparse.Tags, error) {
		name := args[0].(*eparse.StringNode).Text
		lookup := c.Lookups[name]
		if lookup == nil {
			return nil, fmt.Errorf("bad lookup table %v", name)
		}
		t := make(eparse.Tags)
		for _, v := range lookup.Tags {
			t[v] = struct{}{}
		}
		return t, nil
	}
	getAlertExpr := func(name, key string) (*Alert, *expr.Expr, error) {
		a := c.Alerts[name]
		if a == nil {
			return nil, nil, fmt.Errorf("bad alert name %v", name)
		}
		var e *expr.Expr
		switch key {
		case "crit":
			e = a.Crit
		case "warn":
			e = a.Warn
		default:
			return nil, nil, fmt.Errorf("alert: unsupported key %v", key)
		}
		if e == nil {
			return nil, nil, fmt.Errorf("alert: nil expression")
		}
		return a, e, nil
	}
	alert := func(s *expr.State, T miniprofiler.Timer, name, key string) (results *expr.Results, err error) {
		_, e, err := getAlertExpr(name, key)
		if err != nil {
			return nil, err
		}
		results, _, err = e.ExecuteState(s, T)
		return
	}
	tagAlert := func(args []eparse.Node) (eparse.Tags, error) {
		name := args[0].(*eparse.StringNode).Text
		key := args[1].(*eparse.StringNode).Text
		a, e, err := getAlertExpr(name, key)
		if err != nil {
			return nil, err
		}
		if a.returnType != eparse.TypeNumber {
			return nil, fmt.Errorf("alert requires a number-returning expression (got %v)", a.returnType)
		}
		return e.Root.Tags()
	}
	return map[string]eparse.Func{
		"alert": {
			Args:   []eparse.FuncType{eparse.TypeString, eparse.TypeString},
			Return: eparse.TypeNumber,
			Tags:   tagAlert,
			F:      alert,
		},
		"lookup": {
			Args:   []eparse.FuncType{eparse.TypeString, eparse.TypeString},
			Return: eparse.TypeNumber,
			Tags:   tagLookup,
			F:      lookup,
		},
	}
}
