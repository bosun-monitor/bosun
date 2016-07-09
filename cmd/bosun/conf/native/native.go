package native

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"bosun.org/graphite"
	"bosun.org/models"
	"bosun.org/slog"

	htemplate "html/template"
	ttemplate "text/template"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/conf/native/parse"
	"bosun.org/cmd/bosun/expr"
	eparse "bosun.org/cmd/bosun/expr/parse"

	"bosun.org/opentsdb"
	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/influxdata/influxdb/client"
)

type NativeConf struct {
	Vars            conf.Vars
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
	UnknownTemplate  *conf.Template
	UnknownThreshold int
	Templates        map[string]*conf.Template
	Alerts           map[string]*conf.Alert
	Notifications    map[string]*conf.Notification `json:"-"`
	RawText          string
	Macros           map[string]*conf.Macro
	Lookups          map[string]*conf.Lookup
	Squelch          conf.Squelches `json:"-"`
	Quiet            bool
	SkipLast         bool
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

	reload func() error

	tree            *parse.Tree
	node            parse.Node
	unknownTemplate string
	bodies          *htemplate.Template
	subjects        *ttemplate.Template
	squelch         []string

	writeLock chan bool
}

// TSDBContext returns an OpenTSDB context limited to
// c.ResponseLimit. A nil context is returned if TSDBHost is not set.
func (c *NativeConf) GetTSDBContext() opentsdb.Context {
	if c.TSDBHost == "" {
		return nil
	}
	return opentsdb.NewLimitContext(c.TSDBHost, c.ResponseLimit, *c.TSDBVersion)
}

// GraphiteContext returns a Graphite context. A nil context is returned if
// GraphiteHost is not set.
func (c *NativeConf) GetGraphiteContext() graphite.Context {
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

func (c *NativeConf) AlertSquelched(a *conf.Alert) func(opentsdb.TagSet) bool {
	return func(tags opentsdb.TagSet) bool {
		return c.Squelched(a, tags)
	}
}

func (c *NativeConf) Squelched(a *conf.Alert, tags opentsdb.TagSet) bool {
	return c.Squelch.Squelched(tags) || a.Squelch.Squelched(tags)
}

// at marks the state to be on node n, for error reporting.
func (c *NativeConf) at(node parse.Node) {
	c.node = node
}

func (c *NativeConf) error(err error) {
	c.errorf(err.Error())
}

func (c *NativeConf) AnnotateEnabled() bool {
	return len(c.AnnotateElasticHosts) != 0
}

// errorf formats the error and terminates processing.
func (c *NativeConf) errorf(format string, args ...interface{}) {
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

// parseNotifications parses the comma-separated string v for notifications and
// returns them.
func (c *NativeConf) parseNotifications(v string) (map[string]*conf.Notification, error) {
	ns := make(map[string]*conf.Notification)
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

func ParseFile(fname string) (*NativeConf, error) {
	f, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	return NewNativeConf(fname, string(f))
}

func (c *NativeConf) SaveConf(newConf *NativeConf) error {
	return ioutil.WriteFile(c.Name, []byte(newConf.RawText), os.FileMode(int(0640)))
}

func NewNativeConf(name, text string) (c *NativeConf, err error) {
	defer errRecover(&err)
	c = &NativeConf{
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
		Templates:        make(map[string]*conf.Template),
		Alerts:           make(map[string]*conf.Alert),
		Notifications:    make(map[string]*conf.Notification),
		RawText:          text,
		bodies:           htemplate.New(name).Funcs(htemplate.FuncMap(defaultFuncs)),
		subjects:         ttemplate.New(name).Funcs(defaultFuncs),
		Lookups:          make(map[string]*conf.Lookup),
		Macros:           make(map[string]*conf.Macro),
		writeLock:        make(chan bool, 1),
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

func (c *NativeConf) loadGlobal(p *parse.PairNode) {
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

func (c *NativeConf) loadSection(s *parse.SectionNode) {
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

func (c *NativeConf) getPairs(s *parse.SectionNode, vars conf.Vars, st sectionType) (pairs []nodePair) {
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
				for _, p := range m.Pairs.([]nodePair) {
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

func (c *NativeConf) loadLookup(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Lookups[name]; ok {
		c.errorf("duplicate lookup name: %s", name)
	}
	l := conf.Lookup{
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
			e := conf.Entry{
				Def:  n.RawText,
				Name: n.Name.Text,
				ExprEntry: &conf.ExprEntry{
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

func (c *NativeConf) loadMacro(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Macros[name]; ok {
		c.errorf("duplicate macro name: %s", name)
	}
	m := conf.Macro{
		Name: name,
	}
	m.Text = s.RawText
	pairs := c.getPairs(s, nil, sMacro)
	for _, p := range pairs {
		if _, ok := m.Pairs.([]nodePair); !ok { //bad
			m.Pairs = []nodePair{}
		}
		m.Pairs = append(m.Pairs.([]nodePair), p) // bad
	}
	c.at(s)
	c.Macros[name] = &m
}

var defaultFuncs = ttemplate.FuncMap{
	"bytes": func(v interface{}) (conf.ByteSize, error) {
		switch v := v.(type) {
		case string:
			f, err := strconv.ParseFloat(v, 64)
			return conf.ByteSize(f), err
		case int:
			return conf.ByteSize(v), nil
		case float64:
			return conf.ByteSize(v), nil
		case expr.Number:
			return conf.ByteSize(v), nil
		case expr.Scalar:
			return conf.ByteSize(v), nil
		}
		return conf.ByteSize(0), fmt.Errorf("unexpected type: %T (%v)", v, v)
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

func (c *NativeConf) loadTemplate(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Templates[name]; ok {
		c.errorf("duplicate template name: %s", name)
	}
	t := conf.Template{
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
				t.RawBody = v
				tmpl := c.bodies.New(name).Funcs(htemplate.FuncMap(funcs))
				_, err := tmpl.Parse(t.RawBody)
				if err != nil {
					c.error(err)
				}
				t.Body = tmpl
			case "subject":
				t.RawSubject = v
				tmpl := c.subjects.New(name).Funcs(funcs)
				_, err := tmpl.Parse(t.RawSubject)
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

func (c *NativeConf) SetAlert(name, alertText string, hash string) (string, error) {
	select {
	case c.writeLock <- true:
		// Got Write Lock
	default:
		return "", fmt.Errorf("cannot write alert, write in progress")
	}
	defer func() {
		<-c.writeLock
	}()
	// TODO check hash to make sure it matches what the running config is
	a := c.GetAlert(name)
	var newRawConf bytes.Buffer
	if a == nil {
		newRawConf.WriteString(c.RawText)
		newRawConf.WriteString("\n")
		newRawConf.WriteString(alertText)
		//Alert Does not exists
	} else {
		newRawConf.WriteString(c.RawText[:getLocationStart(a)])
		newRawConf.WriteString(alertText)
		newRawConf.WriteString(c.RawText[getLocationEnd(a):])
	}
	newConf, err := NewNativeConf(c.Name, newRawConf.String())
	if err != nil {
		return "", fmt.Errorf("new config not valid: %v", err)
	}
	if err := c.SaveConf(newConf); err != nil {
		return "", fmt.Errorf("couldn't save config file")
	}
	err = c.reload()
	if err != nil {
		return "", err
	}
	return "reloaded", nil
}

func setLocation(a *conf.Alert, start int, end int) {
	a.Location = conf.NativeLocator{start, end}
	a.LocatorType = conf.TypeNative
}

func getLocationStart(a *conf.Alert) int {
	return a.Location.(conf.NativeLocator)[0]
}

func getLocationEnd(a *conf.Alert) int {
	return a.Location.(conf.NativeLocator)[1]
}

func (c *NativeConf) loadAlert(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Alerts[name]; ok {
		c.errorf("duplicate alert name: %s", name)
	}
	a := conf.Alert{
		Vars:             make(map[string]string),
		Name:             name,
		CritNotification: new(conf.Notifications),
		WarnNotification: new(conf.Notifications),
		Locator:          new(conf.Locator),
	}
	a.Text = s.RawText
	start := int(s.Position())
	end := int(s.Position()) + len(s.RawText)
	setLocation(&a, start, end)
	procNotification := func(v string, ns *conf.Notifications) {
		if lookup := lookupNotificationRE.FindStringSubmatch(v); lookup != nil {
			if ns.Lookups == nil {
				ns.Lookups = make(map[string]*conf.Lookup)
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
			ns.Notifications = make(map[string]*conf.Notification)
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
			a.TemplateName = v
			t, ok := c.Templates[a.TemplateName]
			if !ok {
				c.errorf("template not found %s", a.TemplateName)
			}
			a.Template = t
		case "crit":
			a.Crit = c.NewExpr(v)
		case "warn":
			a.Warn = c.NewExpr(v)
		case "depends":
			a.Depends = c.NewExpr(v)
		case "squelch":
			a.RawSquelch = append(a.RawSquelch, v)
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
	a.ReturnType = ret
	c.Alerts[name] = &a
}

func (c *NativeConf) loadNotification(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Notifications[name]; ok {
		c.errorf("duplicate notification name: %s", name)
	}
	n := conf.Notification{
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
			n.RawEmail = v
			email, err := mail.ParseAddressList(n.RawEmail)
			if err != nil {
				c.error(err)
			}
			n.Email = email
		case "post":
			n.RawPost = v
			post, err := url.Parse(n.RawPost)
			if err != nil {
				c.error(err)
			}
			n.Post = post
		case "get":
			n.RawGet = v
			get, err := url.Parse(n.RawGet)
			if err != nil {
				c.error(err)
			}
			n.Get = get
		case "print":
			n.Print = true
		case "contentType":
			n.ContentType = v
		case "next":
			n.NextName = v
			next, ok := c.Notifications[n.NextName]
			if !ok {
				c.errorf("unknown notification %s", n.NextName)
			}
			n.Next = next
		case "timeout":
			d, err := opentsdb.ParseDuration(v)
			if err != nil {
				c.error(err)
			}
			n.Timeout = time.Duration(d)
		case "body":
			n.RawBody = v
			tmpl := ttemplate.New(name).Funcs(funcs)
			_, err := tmpl.Parse(n.RawBody)
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

func (c *NativeConf) Expand(v string, vars map[string]string, ignoreBadExpand bool) string {
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

func (c *NativeConf) seen(v string, m map[string]bool) {
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

func (c *NativeConf) NewExpr(s string) *expr.Expr {
	exp, err := expr.New(s, c.GetFuncs())
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

func (c *NativeConf) GetFuncs() map[string]eparse.Func {
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
		if a.ReturnType != models.TypeNumberSet {
			return nil, fmt.Errorf("alert requires a number-returning expression (got %v)", a.ReturnType)
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

func (c *NativeConf) getAlertExpr(name, key string) (*conf.Alert, *expr.Expr, error) {
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

func (c *NativeConf) alert(s *expr.State, T miniprofiler.Timer, name, key string) (results *expr.Results, err error) {
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

func (c *NativeConf) MakeLink(path string, v *url.Values) string {
	u := url.URL{
		Scheme:   "http",
		Host:     c.Hostname,
		Path:     path,
		RawQuery: v.Encode(),
	}
	return u.String()
}

func (c *NativeConf) GetCheckFrequency() time.Duration {
	return c.CheckFrequency
}

func (c *NativeConf) GetHTTPListen() string {
	return c.HTTPListen
}

func (c *NativeConf) GetHostname() string {
	return c.Hostname
}

func (c *NativeConf) GetRelayListen() string {
	return c.RelayListen
}

func (c *NativeConf) GetSMTPHost() string {
	return c.SMTPHost
}

func (c *NativeConf) GetSMTPUsername() string {
	return c.SMTPUsername
}

func (c *NativeConf) GetSMTPPassword() string {
	return c.SMTPPassword
}

func (c *NativeConf) GetPing() bool {
	return c.Ping
}

func (c *NativeConf) GetPingDuration() time.Duration {
	return c.PingDuration
}

func (c *NativeConf) GetEmailFrom() string {
	return c.EmailFrom
}

func (c *NativeConf) GetLedisDir() string {
	return c.LedisDir
}

func (c *NativeConf) GetLedisBindAddr() string {
	return c.LedisBindAddr
}

func (c *NativeConf) GetRedisHost() string {
	return c.RedisHost
}

func (c *NativeConf) GetRedisDb() int {
	return c.RedisDb
}

func (c *NativeConf) GetRedisPassword() string {
	return c.RedisPassword
}

func (c *NativeConf) GetTimeAndDate() []int {
	return c.TimeAndDate
}

func (c *NativeConf) GetResponseLimit() int64 {
	return c.ResponseLimit
}

func (c *NativeConf) GetSearchSince() opentsdb.Duration {
	return c.SearchSince
}

func (c *NativeConf) SetQuiet(quiet bool) {
	c.Quiet = quiet
}

func (c *NativeConf) GetQuiet() bool {
	return c.Quiet
}

func (c *NativeConf) SetSkipLast(skipLast bool) {
	c.SkipLast = skipLast
}

func (c *NativeConf) GetSkipLast() bool {
	return c.SkipLast
}

func (c *NativeConf) GetNoSleep() bool {
	return c.NoSleep
}

func (c *NativeConf) GetShortURLKey() string {
	return c.ShortURLKey
}

func (c *NativeConf) GetInternetProxy() string {
	return c.InternetProxy
}

func (c *NativeConf) GetMinGroupSize() int {
	return c.MinGroupSize
}

func (c *NativeConf) GetUnknownTemplate() *conf.Template {
	return c.UnknownTemplate
}

func (c *NativeConf) GetUnknownThreshold() int {
	return c.UnknownThreshold
}

func (c *NativeConf) GetTemplate(s string) *conf.Template {
	return c.Templates[s]
}

func (c *NativeConf) GetAlerts() map[string]*conf.Alert {
	return c.Alerts
}

func (c *NativeConf) GetAlert(s string) *conf.Alert {
	return c.Alerts[s]
}

func (c *NativeConf) GetNotifications() map[string]*conf.Notification {
	return c.Notifications
}

func (c *NativeConf) GetNotification(s string) *conf.Notification {
	return c.Notifications[s]
}

func (c *NativeConf) GetMacro(s string) *conf.Macro {
	return c.Macros[s]
}
func (c *NativeConf) GetLookup(s string) *conf.Lookup {
	return c.Lookups[s]
}
func (c *NativeConf) GetSquelches() conf.Squelches {
	return c.Squelch
}

func (c *NativeConf) SetTSDBHost(tsdbHost string) {
	c.TSDBHost = tsdbHost
}

func (c *NativeConf) GetTSDBHost() string {
	return c.TSDBHost
}
func (c *NativeConf) GetTSDBVersion() *opentsdb.Version {
	return c.TSDBVersion
}

func (c *NativeConf) GetGraphiteHost() string {
	return c.GraphiteHost
}

func (c *NativeConf) GetGraphiteHeaders() []string {
	return c.GraphiteHeaders
}

func (c *NativeConf) GetLogstashElasticHosts() expr.LogstashElasticHosts {
	return c.LogstashElasticHosts
}

func (c *NativeConf) GetAnnotateElasticHosts() expr.ElasticHosts {
	return c.AnnotateElasticHosts
}

func (c *NativeConf) GetAnnotateIndex() string {
	return c.AnnotateIndex
}

func (c *NativeConf) GetInfluxContext() client.Config {
	return c.InfluxConfig
}

func (c *NativeConf) GetLogstashContext() expr.LogstashElasticHosts {
	return c.LogstashElasticHosts
}

func (c *NativeConf) GetElasticContext() expr.ElasticHosts {
	return c.ElasticHosts
}

func (c *NativeConf) GetRawText() string {
	return c.RawText
}

func (c *NativeConf) SetReload(reload func() error) {
	c.reload = reload
}

func (c *NativeConf) Reload() error {
	return c.reload()
}
