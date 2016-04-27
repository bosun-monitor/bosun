package conf // import "bosun.org/cmd/bosun/conf"

import (
	"encoding/json"
	"fmt"
	htemplate "html/template"
	"io/ioutil"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	ttemplate "text/template"
	"time"

	"bosun.org/cmd/bosun/conf/parse"
	"bosun.org/cmd/bosun/expr"
	eparse "bosun.org/cmd/bosun/expr/parse"
	"bosun.org/graphite"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/influxdata/influxdb/client"
)

type Conf struct {
	Vars
	Name            string        // Config file name
	CheckFrequency  time.Duration // Time between alert checks: 5m
	DefaultRunEvery int           // Default number of check intervals to run each alert: 1
	HTTPListen      string        // Web server listen address: :80
	Hostname        string
	RelayListen     string // OpenTSDB relay listen address: :4242
	SMTPHost        string // SMTP address: ny-mail:25
	SMTPUsername    string // SMTP username
	SMTPPassword    string // SMTP password
	Ping            bool
	PingDuration    time.Duration // Duration from now to stop pinging hosts based on time since the host tag was touched
	EmailFrom       string
	StateFile       string
	LedisDir        string
	LedisBindAddr   string

	RedisHost     string
	RedisDb       int
	RedisPassword string

	TimeAndDate      []int // timeanddate.com cities list
	ResponseLimit    int64
	SearchSince      opentsdb.Duration
	UnknownTemplate  *Template
	UnknownThreshold int
	Templates        map[string]*Template
	Alerts           map[string]*Alert
	Notifications    map[string]*Notification `json:"-"`
	RawText          string
	Macros           map[string]*Macro
	Lookups          map[string]*Lookup
	Squelch          Squelches `json:"-"`
	Quiet            bool
	NoSleep          bool
	ShortURLKey      string
	InternetProxy    string
	MinGroupSize     int

	TSDBHost             string                    // OpenTSDB relay and query destination: ny-devtsdb04:4242
	TSDBVersion          *opentsdb.Version         // If set to 2.2 , enable passthrough of wildcards and filters, and add support for groupby
	GraphiteHost         string                    // Graphite query host: foo.bar.baz
	GraphiteHeaders      []string                  // extra http headers when querying graphite.
	LogstashElasticHosts expr.LogstashElasticHosts // CSV Elastic Hosts (All part of the same cluster) that stores logstash documents, i.e http://ny-elastic01:9200. Only works with elastc pre-v2, and expects the schema to be logstash's default.
	ElasticHosts         expr.ElasticHosts         // CSV Elastic Hosts (All part of the same cluster), i.e http://ny-elastic01:9200. Only works with elastic v2+, and unlike logstash it is designed to be able to use various elastic schemas.
	InfluxConfig         client.Config

	AnnotateElasticHosts []string // CSV of Elastic Hosts, currently the only backend in annotate
	AnnotateIndex        string   // name of index / table

	tree            *parse.Tree
	node            parse.Node
	unknownTemplate string
	bodies          *htemplate.Template
	subjects        *ttemplate.Template
	squelch         []string
}

// TSDBContext returns an OpenTSDB context limited to
// c.ResponseLimit. A nil context is returned if TSDBHost is not set.
func (c *Conf) TSDBContext() opentsdb.Context {
	if c.TSDBHost == "" {
		return nil
	}
	return opentsdb.NewLimitContext(c.TSDBHost, c.ResponseLimit, *c.TSDBVersion)
}

// GraphiteContext returns a Graphite context. A nil context is returned if
// GraphiteHost is not set.
func (c *Conf) GraphiteContext() graphite.Context {
	if c.GraphiteHost == "" {
		return nil
	}
	if len(c.GraphiteHeaders) > 0 {
		headers := http.Header(make(map[string][]string))
		for _, s := range c.GraphiteHeaders {
			kv := strings.Split(s, ":")
			headers.Add(kv[0], kv[1])
		}
		return graphite.HostHeader{
			Host:   c.GraphiteHost,
			Header: headers,
		}
	}
	return graphite.Host(c.GraphiteHost)
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

func (c *Conf) AnnotateEnabled() bool {
	return len(c.AnnotateElasticHosts) != 0
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
	Text    string
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
	Text  string
	Pairs []nodePair
	Name  string
}

type Alert struct {
	Text string
	Vars
	*Template        `json:"-"`
	Name             string
	Crit             *expr.Expr `json:",omitempty"`
	Warn             *expr.Expr `json:",omitempty"`
	Depends          *expr.Expr `json:",omitempty"`
	Squelch          Squelches  `json:"-"`
	CritNotification *Notifications
	WarnNotification *Notifications
	Unknown          time.Duration
	MaxLogFrequency  time.Duration
	IgnoreUnknown    bool
	UnknownsNormal   bool
	UnjoinedOK       bool `json:",omitempty"`
	Log              bool
	RunEvery         int
	returnType       models.FuncType

	template string
	squelch  []string
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
	Text string
	Vars
	Name    string
	Body    *htemplate.Template `json:"-"`
	Subject *ttemplate.Template `json:"-"`

	body, subject string
}

type Notification struct {
	Text string
	Vars
	Name         string
	Email        []*mail.Address
	Post, Get    *url.URL
	Body         *ttemplate.Template
	Print        bool
	Next         *Notification
	Timeout      time.Duration
	ContentType  string
	RunOnActions bool
	UseBody      bool

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
		Name:             name,
		CheckFrequency:   time.Minute * 5,
		DefaultRunEvery:  1,
		HTTPListen:       ":8070",
		StateFile:        "bosun.state",
		LedisDir:         "ledis_data",
		LedisBindAddr:    "127.0.0.1:9565",
		MinGroupSize:     5,
		PingDuration:     time.Hour * 24,
		ResponseLimit:    1 << 20, // 1MB
		SearchSince:      opentsdb.Day * 3,
		TSDBVersion:      &opentsdb.Version2_1,
		UnknownThreshold: 5,
		Vars:             make(map[string]string),
		Templates:        make(map[string]*Template),
		Alerts:           make(map[string]*Alert),
		Notifications:    make(map[string]*Notification),
		RawText:          text,
		bodies:           htemplate.New(name).Funcs(htemplate.FuncMap(defaultFuncs)),
		subjects:         ttemplate.New(name).Funcs(defaultFuncs),
		Lookups:          make(map[string]*Lookup),
		Macros:           make(map[string]*Macro),
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
		if !strings.Contains(v, ":") && v != "" {
			v += ":4242"
		}
		c.TSDBHost = v
	case "tsdbVersion":
		sp := strings.Split(v, ".")
		if len(sp) != 2 {
			c.errorf("tsdbVersion must be in number.number form")
		}
		major, err := strconv.ParseInt(sp[0], 10, 64)
		if err != nil {
			c.errorf("error pasing opentsdb major version number %v: %v", sp[0], err)
		}
		minor, err := strconv.ParseInt(sp[1], 10, 64)
		if err != nil {
			c.errorf("error pasing opentsdb minor version number %v: %v", sp[1], err)
		}
		c.TSDBVersion = &opentsdb.Version{Major: major, Minor: minor}
	case "graphiteHost":
		c.GraphiteHost = v
	case "graphiteHeader":
		if !strings.Contains(v, ":") {
			c.errorf("graphiteHeader must be in key:value form")
		}
		c.GraphiteHeaders = append(c.GraphiteHeaders, v)
	case "logstashElasticHosts":
		c.LogstashElasticHosts = strings.Split(v, ",")
	case "elasticHosts":
		c.ElasticHosts = strings.Split(v, ",")
	case "influxHost":
		c.InfluxConfig.URL.Host = v
		c.InfluxConfig.UserAgent = "bosun"
		// Default scheme to non-TLS
		c.InfluxConfig.URL.Scheme = "http"
	case "influxUsername":
		c.InfluxConfig.Username = v
	case "influxPassword":
		c.InfluxConfig.Password = v
	case "influxTLS":
		b, err := strconv.ParseBool(v)
		if err != nil {
			c.error(err)
		}
		if b {
			c.InfluxConfig.URL.Scheme = "https"
		} else {
			c.InfluxConfig.URL.Scheme = "http"
		}
	case "influxTimeout":
		od, err := opentsdb.ParseDuration(v)
		if err != nil {
			c.error(err)
		}
		d := time.Duration(od)
		c.InfluxConfig.Timeout = d
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
	case "pingDuration":
		d, err := time.ParseDuration(v)
		if err != nil {
			c.errorf(err.Error())
		}
		c.PingDuration = d
	case "noSleep":
		c.NoSleep = true
	case "unknownThreshold":
		i, err := strconv.Atoi(v)
		if err != nil {
			c.error(err)
		}
		c.UnknownThreshold = i
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
	case "defaultRunEvery":
		var err error
		c.DefaultRunEvery, err = strconv.Atoi(v)
		if err != nil {
			c.error(err)
		}
		if c.DefaultRunEvery <= 0 {
			c.errorf("defaultRunEvery must be > 0")
		}
	case "searchSince":
		s, err := opentsdb.ParseDuration(v)
		if err != nil {
			c.error(err)
		}
		c.SearchSince = s
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
	case "shortURLKey":
		c.ShortURLKey = v
	case "internetProxy":
		c.InternetProxy = v
	case "ledisDir":
		c.LedisDir = v
	case "ledisBindAddr":
		c.LedisBindAddr = v
	case "redisHost":
		c.RedisHost = v
	case "redisPassword":
		c.RedisPassword = v
	case "redisDb":
		i, err := strconv.Atoi(v)
		if err != nil {
			c.error(err)
		}
		c.RedisDb = i
	case "annotateElasticHosts":
		c.AnnotateElasticHosts = strings.Split(v, ",")
	case "annotationIndex":
		c.AnnotateIndex = v
	case "minGroupSize":
		i, err := strconv.Atoi(v)
		if err != nil {
			c.error(err)
		}
		c.MinGroupSize = i
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

func (c *Conf) parseIPs(s string) (nets []*net.IPNet) {
	rawCIDRs := strings.Split(s, ",")
	for _, rc := range rawCIDRs {
		_, ipnet, err := net.ParseCIDR(rc)
		if err != nil {
			c.error(err)
		}
		nets = append(nets, ipnet)
	}
	return nets
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

func (c *Conf) getPairs(s *parse.SectionNode, vars Vars, st sectionType) (pairs []nodePair) {
	saw := make(map[string]bool)
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
	return
}

func (c *Conf) loadLookup(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Lookups[name]; ok {
		c.errorf("duplicate lookup name: %s", name)
	}
	l := Lookup{
		Name: name,
	}
	l.Text = s.RawText
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
					AlertKey: models.NewAlertKey("", tags),
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
		Name: name,
	}
	m.Text = s.RawText
	pairs := c.getPairs(s, nil, sMacro)
	for _, p := range pairs {
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
	"pct": func(i interface{}) string {
		return fmt.Sprintf("%.2f%%", i)
	},
	"replace": strings.Replace,
	"short": func(v string) string {
		return strings.SplitN(v, ".", 2)[0]
	},
	"html": func(value interface{}) htemplate.HTML {
		return htemplate.HTML(fmt.Sprint(value))
	},
	"parseDuration": time.ParseDuration,
}

func (c *Conf) loadTemplate(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Templates[name]; ok {
		c.errorf("duplicate template name: %s", name)
	}
	t := Template{
		Vars: make(map[string]string),
		Name: name,
	}
	t.Text = s.RawText
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
		Vars:             make(map[string]string),
		Name:             name,
		CritNotification: new(Notifications),
		WarnNotification: new(Notifications),
	}
	a.Text = s.RawText
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
	pairs := c.getPairs(s, a.Vars, sNormal)
	for _, p := range pairs {
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
			a.Crit = c.NewExpr(v)
		case "warn":
			a.Warn = c.NewExpr(v)
		case "depends":
			a.Depends = c.NewExpr(v)
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
		case "maxLogFrequency":
			od, err := opentsdb.ParseDuration(v)
			if err != nil {
				c.error(err)
			}
			d := time.Duration(od)
			if d < time.Second {
				c.errorf("max log frequency must be at least 1s")
			}
			a.MaxLogFrequency = d
		case "unjoinedOk":
			a.UnjoinedOK = true
		case "ignoreUnknown":
			a.IgnoreUnknown = true
		case "unknownIsNormal":
			a.UnknownsNormal = true
		case "log":
			a.Log = true
		case "runEvery":
			var err error
			a.RunEvery, err = strconv.Atoi(v)
			if err != nil {
				c.error(err)
			}
		default:
			c.errorf("unknown key %s", p.key)
		}
	}
	if a.MaxLogFrequency != 0 && !a.Log {
		c.errorf("maxLogFrequency can only be used on alerts with `log = true`.")
	}
	c.at(s)
	if a.Crit == nil && a.Warn == nil {
		c.errorf("neither crit or warn specified")
	}
	var tags eparse.Tags
	var ret models.FuncType
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
	if a.Depends != nil {
		depTags, err := a.Depends.Root.Tags()
		if err != nil {
			c.error(err)
		}
		if len(depTags.Intersection(tags)) < 1 {
			c.errorf("Depends and crit/warn must share at least one tag.")
		}
	}
	if a.Log {
		for _, n := range a.CritNotification.Notifications {
			if n.Next != nil {
				c.errorf("cannot use log with a chained notification")
			}
		}
		for _, n := range a.WarnNotification.Notifications {
			if n.Next != nil {
				c.errorf("cannot use log with a chained notification")
			}
		}
		if a.Crit != nil && len(a.CritNotification.Notifications) == 0 {
			c.errorf("log + crit specified, but no critNotification")
		}
		if a.Warn != nil && len(a.WarnNotification.Notifications) == 0 {
			c.errorf("log + warn specified, but no warnNotification")
		}
	}
	if len(a.WarnNotification.Notifications) != 0 {
		if a.Warn == nil {
			c.errorf("warnNotification specified, but no warn")
		}
		if a.Template == nil {
			c.errorf("warnNotification specified, but no template")
		}
	}
	if len(a.CritNotification.Notifications) != 0 {
		if a.Crit == nil {
			c.errorf("critNotification specified, but no crit")
		}
		if a.Template == nil {
			c.errorf("critNotification specified, but no template")
		}
	}
	if a.RunEvery == 0 {
		a.RunEvery = c.DefaultRunEvery
	}
	a.returnType = ret
	c.Alerts[name] = &a
}

func (c *Conf) loadNotification(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Notifications[name]; ok {
		c.errorf("duplicate notification name: %s", name)
	}
	n := Notification{
		Vars:         make(map[string]string),
		ContentType:  "application/x-www-form-urlencoded",
		Name:         name,
		RunOnActions: true,
	}
	n.Text = s.RawText
	funcs := ttemplate.FuncMap{
		"V": func(v string) string {
			return c.Expand(v, n.Vars, false)
		},
		"json": func(v interface{}) string {
			b, err := json.Marshal(v)
			if err != nil {
				slog.Errorln(err)
			}
			return string(b)
		},
	}
	c.Notifications[name] = &n
	pairs := c.getPairs(s, n.Vars, sNormal)
	for _, p := range pairs {
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
		case "contentType":
			n.ContentType = v
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
		case "runOnActions":
			n.RunOnActions = v == "true"
		case "useBody":
			n.UseBody = v == "true"
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
		if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") && !ignoreBadExpand {
			s = "$" + s[2:len(s)-1]
		}
		if _n, ok := vars[s]; ok {
			n = _n
		} else if _n, ok := c.Vars[s]; ok {
			n = _n
		} else if strings.HasPrefix(s, "$env.") {
			n = os.Getenv(s[5:])
		} else if ignoreBadExpand {
			return s
		} else {
			c.errorf("unknown variable %s", s)
		}
		return c.Expand(n, vars, ignoreBadExpand)
	})
	return ss
}

func (c *Conf) seen(v string, m map[string]bool) {
	if m[v] {
		switch v {
		case "squelch", "critNotification", "warnNotification", "graphiteHeader":
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

func (c *Conf) NewExpr(s string) *expr.Expr {
	exp, err := expr.New(s, c.Funcs())
	if err != nil {
		c.error(err)
	}
	switch exp.Root.Return() {
	case models.TypeNumberSet, models.TypeScalar:
		break
	default:
		c.errorf("expression must return a number")
	}
	return exp
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
			vals, err := e.Search.TagValuesByTagKey(tag, 0)
			if err != nil {
				return nil, err
			}
			for _, value := range vals {
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
	lookupSeries := func(e *expr.State, T miniprofiler.Timer, series *expr.Results, lookup, key string) (results *expr.Results, err error) {
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
		for _, res := range series.Results {
			value, ok := lookups.Get(key, res.Group)
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
				Group: res.Group,
			})
		}
		return results, nil
	}
	lookupTags := func(args []eparse.Node) (eparse.Tags, error) {
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
	lookupSeriesTags := func(args []eparse.Node) (eparse.Tags, error) {
		name := args[1].(*eparse.StringNode).Text
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

	tagAlert := func(args []eparse.Node) (eparse.Tags, error) {
		name := args[0].(*eparse.StringNode).Text
		key := args[1].(*eparse.StringNode).Text
		a, e, err := c.getAlertExpr(name, key)
		if err != nil {
			return nil, err
		}
		if a.returnType != models.TypeNumberSet {
			return nil, fmt.Errorf("alert requires a number-returning expression (got %v)", a.returnType)
		}
		return e.Root.Tags()
	}

	funcs := map[string]eparse.Func{
		"alert": {
			Args:   []models.FuncType{models.TypeString, models.TypeString},
			Return: models.TypeNumberSet,
			Tags:   tagAlert,
			F:      c.alert,
		},
		"lookup": {
			Args:   []models.FuncType{models.TypeString, models.TypeString},
			Return: models.TypeNumberSet,
			Tags:   lookupTags,
			F:      lookup,
		},
		"lookupSeries": {
			Args:   []models.FuncType{models.TypeSeriesSet, models.TypeString, models.TypeString},
			Return: models.TypeNumberSet,
			Tags:   lookupSeriesTags,
			F:      lookupSeries,
		},
	}
	merge := func(fs map[string]eparse.Func) {
		for k, v := range fs {
			funcs[k] = v
		}
	}
	if c.TSDBHost != "" {
		merge(expr.TSDB)
	}
	if c.GraphiteHost != "" {
		merge(expr.Graphite)
	}
	if len(c.LogstashElasticHosts) != 0 {
		merge(expr.LogstashElastic)
	}
	if len(c.ElasticHosts) != 0 {
		merge(expr.Elastic)
	}
	if c.InfluxConfig.URL.Host != "" {
		merge(expr.Influx)
	}
	return funcs
}

func (c *Conf) getAlertExpr(name, key string) (*Alert, *expr.Expr, error) {
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

func (c *Conf) alert(s *expr.State, T miniprofiler.Timer, name, key string) (results *expr.Results, err error) {
	_, e, err := c.getAlertExpr(name, key)
	if err != nil {
		return nil, err
	}
	results, _, err = e.ExecuteState(s, T)
	if err != nil {
		return nil, err
	}
	if s.History != nil {
		unknownTags, unevalTags := s.History.GetUnknownAndUnevaluatedAlertKeys(name)
		// For currently unknown tags NOT in the result set, add an error result
		for _, ak := range unknownTags {
			found := false
			for _, result := range results.Results {
				if result.Group.Equal(ak.Group()) {
					found = true
					break
				}
			}
			if !found {
				res := expr.Result{
					Value: expr.Number(1),
					Group: ak.Group(),
				}
				results.Results = append(results.Results, &res)
			}
		}
		//For all unevaluated tags in run history, make sure we report a nonzero result.
		for _, ak := range unevalTags {
			found := false
			for _, result := range results.Results {
				if result.Group.Equal(ak.Group()) {
					result.Value = expr.Number(1)
					found = true
					break
				}
			}
			if !found {
				res := expr.Result{
					Value: expr.Number(1),
					Group: ak.Group(),
				}
				results.Results = append(results.Results, &res)
			}
		}
	}
	return results, nil
}

func (c *Conf) MakeLink(path string, v *url.Values) string {
	u := url.URL{
		Scheme:   "http",
		Host:     c.Hostname,
		Path:     path,
		RawQuery: v.Encode(),
	}
	return u.String()
}
