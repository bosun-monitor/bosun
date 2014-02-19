package conf

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/mail"
	"net/url"
	"regexp"
	"runtime"
	"strings"
	"text/template"
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
	*Template    `json:"-"`
	Name         string
	Crit         *expr.Expr `json:",omitempty"`
	Warn         *expr.Expr `json:",omitempty"`
	Squelch      map[string]*regexp.Regexp
	Notification map[string]*Notification

	crit, warn   string
	template     string
	squelch      string
	notification string
}

type Template struct {
	Vars
	Name          string
	Body, Subject *template.Template

	body, subject string
}

type Notification struct {
	Vars
	Name      string
	Email     []*mail.Address
	Post, Get *url.URL
	Next      *Notification
	Timeout   time.Duration

	next      string
	email     string
	post, get string
}

type context struct {
	Alert   *Alert
	Tags    opentsdb.TagSet
	Context opentsdb.Context
}

// E executes the given expression and returns a value with corresponding tags
// to the context's tags. If no such result is found, the first result with nil
// tags is returned. If no such result is found, nil is returned.
func (c *context) E(v string) expr.Value {
	e, err := expr.New(v)
	if err != nil {
		return nil
	}
	res, err := e.Execute(c.Context, nil)
	if err != nil {
		return nil
	}
	for _, r := range res {
		if r.Group.Equal(c.Tags) {
			return r.Value
		}
	}
	for _, r := range res {
		if r.Group == nil {
			return r.Value
		}
	}
	return nil
}

func (a *Alert) data(group opentsdb.TagSet, c opentsdb.Context) interface{} {
	return &context{
		a,
		group,
		c,
	}
}

func (a *Alert) ExecuteBody(w io.Writer, group opentsdb.TagSet, c opentsdb.Context) error {
	if a.Template == nil || a.Template.Body == nil {
		return nil
	}
	return a.Template.Body.Execute(w, a.data(group, c))
}

func (a *Alert) ExecuteSubject(w io.Writer, group opentsdb.TagSet, c opentsdb.Context) error {
	if a.Template == nil || a.Template.Subject == nil {
		return nil
	}
	return a.Template.Subject.Execute(w, a.data(group, c))
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
	v := p.Val.Text
	switch k := p.Key.Text; k {
	case "tsdbHost":
		c.TsdbHost = c.expand(v, nil)
	case "httpListen":
		c.HttpListen = c.expand(v, nil)
	case "relayListen":
		c.RelayListen = c.expand(v, nil)
	case "webDir":
		c.WebDir = c.expand(v, nil)
	case "smtpHost":
		c.SmtpHost = c.expand(v, nil)
	case "emailFrom":
		c.EmailFrom = c.expand(v, nil)
	default:
		if !strings.HasPrefix(k, "$") {
			c.errorf("unknown key %s", k)
		}
		c.Vars[k] = c.expand(v, nil)
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
	V := func(v string) string {
		return c.expand(v, t.Vars)
	}
	master := template.New(name).Funcs(template.FuncMap{
		"V": V,
	})
	for _, p := range s.Nodes {
		c.at(p)
		v := p.Val.Text
		switch k := p.Key.Text; k {
		case "body":
			t.body = v
			tmpl := master.New(k)
			_, err := tmpl.Parse(t.body)
			if err != nil {
				c.error(err)
			}
			t.Body = tmpl
		case "subject":
			t.subject = v
			tmpl := master.New(k)
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
		Vars: make(map[string]string),
		Name: name,
	}
	for _, p := range s.Nodes {
		c.at(p)
		v := p.Val.Text
		switch k := p.Key.Text; k {
		case "template":
			a.template = c.expand(v, a.Vars)
			t, ok := c.Templates[a.template]
			if !ok {
				c.errorf("unknown template %s", a.template)
			}
			a.Template = t
		case "crit":
			a.crit = c.expand(v, a.Vars)
			crit, err := expr.New(a.crit)
			if err != nil {
				c.error(err)
			}
			if crit.Root.Return() != eparse.TYPE_NUMBER {
				c.errorf("crit must return a number")
			}
			a.Crit = crit
		case "warn":
			a.warn = c.expand(v, a.Vars)
			warn, err := expr.New(a.warn)
			if err != nil {
				c.error(err)
			}
			if warn.Root.Return() != eparse.TYPE_NUMBER {
				c.errorf("warn must return a number")
			}
			a.Warn = warn
		case "squelch":
			a.squelch = c.expand(v, a.Vars)
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
		case "notification":
			a.notification = c.expand(v, a.Vars)
			a.Notification = make(map[string]*Notification)
			for _, s := range strings.Split(a.notification, ",") {
				s = strings.TrimSpace(s)
				n, ok := c.Notifications[s]
				if !ok {
					c.errorf("unknown notification %s", s)
				}
				a.Notification[s] = n
			}
		default:
			if !strings.HasPrefix(k, "$") {
				c.errorf("unknown key %s", k)
			}
			a.Vars[k] = c.expand(v, a.Vars)
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
		v := p.Val.Text
		switch k := p.Key.Text; k {
		case "email":
			if c.SmtpHost == "" || c.EmailFrom == "" {
				c.errorf("email notifications require both smtpHost and emailFrom to be set")
			}
			n.email = c.expand(v, n.Vars)
			email, err := mail.ParseAddressList(n.email)
			if err != nil {
				c.error(err)
			}
			n.Email = email
		case "post":
			n.post = c.expand(v, n.Vars)
			post, err := url.Parse(n.post)
			if err != nil {
				c.error(err)
			}
			n.Post = post
		case "get":
			n.get = c.expand(v, n.Vars)
			get, err := url.Parse(n.get)
			if err != nil {
				c.error(err)
			}
			n.Get = get
		case "next":
			n.next = c.expand(v, n.Vars)
			next, ok := c.Notifications[n.next]
			if !ok {
				c.errorf("unknown notification %s", n.next)
			}
			n.Next = next
		case "timeout":
			d, err := time.ParseDuration(c.expand(v, n.Vars))
			if err != nil {
				c.error(err)
			}
			n.Timeout = d
		default:
			if !strings.HasPrefix(k, "$") {
				c.errorf("unknown key %s", k)
			}
			n.Vars[k] = c.expand(v, n.Vars)
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
