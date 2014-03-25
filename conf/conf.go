package conf

import (
	"encoding/json"
	"fmt"
	htemplate "html/template"
	"io/ioutil"
	"net/mail"
	"net/url"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	ttemplate "text/template"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/conf/parse"
	"github.com/StackExchange/tsaf/expr"
	eparse "github.com/StackExchange/tsaf/expr/parse"
)

type Conf struct {
	Vars
	Name          string // Config file name
	WebDir        string // Static content web directory: web
	TsdbHost      string // OpenTSDB relay and query destination: ny-devtsdb04:4242
	RelayListen   string // OpenTSDB relay listen address: :4242
	HttpListen    string // Web server listen address: :80
	SmtpHost      string // SMTP address: ny-mail:25
	EmailFrom     string
	StateFile     string
	Templates     map[string]*Template
	Alerts        map[string]*Alert
	Notifications map[string]*Notification

	tree *parse.Tree
	node parse.Node
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

type Alert struct {
	Vars
	*Template        `json:"-"`
	Name             string
	Crit             *expr.Expr                `json:",omitempty"`
	Warn             *expr.Expr                `json:",omitempty"`
	Squelch          map[string]*regexp.Regexp `json:"-"`
	CritNotification map[string]*Notification  `json:"-"`
	WarnNotification map[string]*Notification  `json:"-"`

	crit, warn       string
	template         string
	squelch          string
	critNotification string
	warnNotification string
}

type Template struct {
	Vars
	Name    string
	Body    *htemplate.Template
	Subject *ttemplate.Template

	body, subject string
}

type Notification struct {
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
	m := make(map[string]interface{})
	if len(n.Vars) > 0 {
		m["Vars"] = n.Vars
	}
	m["Name"] = n.Name
	if n.email != "" {
		m["Email"] = n.email
	}
	if n.post != "" {
		m["Post"] = n.post
	}
	if n.get != "" {
		m["Get"] = n.get
	}
	if n.Print {
		m["Print"] = n.Print
	}
	if n.next != "" {
		m["Next"] = n.next
	}
	if n.Timeout > 0 {
		m["Timeout"] = n.Timeout
	}
	return json.Marshal(m)
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
		Name:          name,
		HttpListen:    ":8070",
		RelayListen:   ":4242",
		WebDir:        "web",
		StateFile:     "tsaf.state",
		Vars:          make(map[string]string),
		Templates:     make(map[string]*Template),
		Alerts:        make(map[string]*Alert),
		Notifications: make(map[string]*Notification),
	}
	c.tree, err = parse.Parse(name, text)
	if err != nil {
		c.error(err)
	}
	for _, n := range c.tree.Root.Nodes {
		c.at(n)
		switch n := n.(type) {
		case *parse.PairNode:
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
	v := c.expand(p.Val.Text, nil)
	switch k := p.Key.Text; k {
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
	default:
		if !strings.HasPrefix(k, "$") {
			c.errorf("unknown key %s", k)
		}
		c.Vars[k] = v
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
	default:
		c.errorf("unknown section type: %s", s.SectionType.Text)
	}
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
	funcs := ttemplate.FuncMap{
		"V": func(v string) string {
			return c.expand(v, t.Vars)
		},
		"bytes": func(v string) ByteSize {
			f, _ := strconv.ParseFloat(v, 64)
			return ByteSize(f)
		},
		"short": func(v string) string {
			return strings.SplitN(v, ".", 2)[0]
		},
	}
	for _, p := range s.Nodes {
		c.at(p)
		v := p.Val.Text
		switch k := p.Key.Text; k {
		case "body":
			c.errEmpty(t.body)
			t.body = v
			tmpl := htemplate.New(k).Funcs(htemplate.FuncMap(funcs))
			_, err := tmpl.Parse(t.body)
			if err != nil {
				c.error(err)
			}
			t.Body = tmpl
		case "subject":
			c.errEmpty(t.subject)
			t.subject = v
			tmpl := ttemplate.New(k).Funcs(funcs)
			_, err := tmpl.Parse(t.subject)
			if err != nil {
				c.error(err)
			}
			t.Subject = tmpl
		default:
			if !strings.HasPrefix(k, "$") {
				c.errorf("unknown key %s", k)
			}
			c.errEmpty(t.Vars[k])
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
		Vars: make(map[string]string),
		Name: name,
	}
	for _, p := range s.Nodes {
		c.at(p)
		v := c.expand(p.Val.Text, a.Vars)
		switch k := p.Key.Text; k {
		case "template":
			c.errEmpty(a.template)
			a.template = v
			t, ok := c.Templates[a.template]
			if !ok {
				c.errorf("unknown template %s", a.template)
			}
			a.Template = t
		case "crit":
			c.errEmpty(a.crit)
			a.crit = v
			crit, err := expr.New(a.crit)
			if err != nil {
				c.error(err)
			}
			if crit.Root.Return() != eparse.TYPE_NUMBER {
				c.errorf("crit must return a number")
			}
			a.Crit = crit
		case "warn":
			c.errEmpty(a.warn)
			a.warn = v
			warn, err := expr.New(a.warn)
			if err != nil {
				c.error(err)
			}
			if warn.Root.Return() != eparse.TYPE_NUMBER {
				c.errorf("warn must return a number")
			}
			a.Warn = warn
		case "squelch":
			c.errEmpty(a.squelch)
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
			c.errEmpty(a.critNotification)
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
			c.errEmpty(a.warnNotification)
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
		default:
			if !strings.HasPrefix(k, "$") {
				c.errorf("unknown key %s", k)
			}
			c.errEmpty(a.Vars[k])
			a.Vars[k] = v
			a.Vars[k[1:]] = a.Vars[k]
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
		Vars: make(map[string]string),
		Name: name,
	}
	c.Notifications[name] = &n
	for _, p := range s.Nodes {
		c.at(p)
		v := c.expand(p.Val.Text, n.Vars)
		switch k := p.Key.Text; k {
		case "email":
			c.errEmpty(n.email)
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
			c.errEmpty(n.post)
			n.post = v
			post, err := url.Parse(n.post)
			if err != nil {
				c.error(err)
			}
			n.Post = post
		case "get":
			c.errEmpty(n.get)
			n.get = v
			get, err := url.Parse(n.get)
			if err != nil {
				c.error(err)
			}
			n.Get = get
		case "print":
			if n.Print {
				c.errEmpty(".")
			}
			n.Print = true
		case "next":
			c.errEmpty(n.next)
			n.next = v
			next, ok := c.Notifications[n.next]
			if !ok {
				c.errorf("unknown notification %s", n.next)
			}
			n.Next = next
		case "timeout":
			if n.Timeout > 0 {
				c.errEmpty(".")
			}
			d, err := time.ParseDuration(v)
			if err != nil {
				c.error(err)
			}
			n.Timeout = d
		default:
			if !strings.HasPrefix(k, "$") {
				c.errorf("unknown key %s", k)
			}
			c.errEmpty(n.Vars[k])
			n.Vars[k] = v
			n.Vars[k[1:]] = n.Vars[k]
		}
	}
	c.at(s)
	if n.Timeout > 0 && n.Next == nil {
		c.errorf("timeout specified without next")
	}
}

var exRE = regexp.MustCompile(`\$\w+`)

func (c *Conf) expand(v string, vars map[string]string) string {
	v = exRE.ReplaceAllStringFunc(v, func(s string) string {
		if vars != nil {
			if n, ok := vars[s]; ok {
				return c.expand(n, vars)
			}
		}
		n, ok := c.Vars[s]
		if !ok {
			c.errorf("unknown variable %s", s)
		}
		return c.expand(n, nil)
	})
	return v
}

// errEmpty panics if v is not "".
func (c *Conf) errEmpty(v string) {
	if v != "" {
		c.errorf("duplicate key")
	}
}
