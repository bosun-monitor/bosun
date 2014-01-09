package conf

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"runtime"
	"strings"
	"text/template"

	"github.com/StackExchange/tsaf/conf/parse"
	"github.com/StackExchange/tsaf/expr"
)

type Conf struct {
	Vars
	Name        string // Config file name
	WebDir      string // Static content web directory: web
	TsdbHost    string // OpenTSDB relay and query destination: ny-devtsdb04:4242
	RelayListen string // OpenTSDB relay listen address: :4242
	HttpListen  string // Web server listen address: :80
	Templates   map[string]*Template
	Alerts      map[string]*Alert

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
	*Template
	Name       string
	Owner      string
	Crit, Warn *expr.Expr
	Overriders []*Alert
	Overrides  *Alert

	crit, warn string
	template   string
	override   string
}

type Template struct {
	Vars
	Name          string
	Body, Subject *template.Template

	body, subject string
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
	t, e := parse.Parse(name, text)
	if e != nil {
		c.error(err)
	}
	c = &Conf{
		tree:        t,
		Name:        name,
		HttpListen:  ":8070",
		RelayListen: ":4242",
		WebDir:      "web",
		Vars:        make(map[string]string),
		Templates:   make(map[string]*Template),
		Alerts:      make(map[string]*Alert),
	}
	for _, n := range t.Root.Nodes {
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
	case "c.webDir":
		c.WebDir = c.expand(v, nil)
	default:
		if !strings.HasPrefix(k, "$") {
			c.errorf("unknown key %s", k)
		}
		c.Vars[k] = c.expand(v, nil)
	}
}

func (c *Conf) loadSection(s *parse.SectionNode) {
	sp := strings.SplitN(s.Name.Text, ".", 2)
	if len(sp) != 2 {
		c.errorf("expected . in section name")
	} else if sp[0] == "template" {
		c.loadTemplate(sp[1], s)
	} else if sp[0] == "alert" {
		c.loadAlert(sp[1], s)
	} else {
		c.errorf("unknown section type: %s", sp[0])
	}
}

func (c *Conf) loadTemplate(name string, s *parse.SectionNode) {
	if _, ok := c.Templates[name]; ok {
		c.errorf("duplicate template name: %s", name)
	}
	t := Template{
		Vars: make(map[string]string),
		Name: name,
	}
	for _, p := range s.Nodes {
		c.at(p)
		v := p.Val.Text
		switch k := p.Key.Text; k {
		case "body":
			t.body = v
			tmpl := template.New(name)
			_, err := tmpl.Parse(t.body)
			if err != nil {
				c.error(err)
			}
			t.Body = tmpl
		case "subject":
			t.subject = v
			tmpl := template.New(name)
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
		}
	}
	c.at(s)
	if t.Body == nil && t.Subject == nil {
		c.errorf("neither body or subject specified")
	}
	c.Templates[name] = &t
}

func (c *Conf) loadAlert(name string, s *parse.SectionNode) {
	if _, ok := c.Alerts[name]; ok {
		c.errorf("duplicate template name: %s", name)
	}
	a := Alert{
		Vars: make(map[string]string),
		Name: name,
	}
	for _, p := range s.Nodes {
		c.at(p)
		v := p.Val.Text
		switch k := p.Key.Text; k {
		case "owner":
			a.Owner = c.expand(v, a.Vars)
		case "template":
			a.template = c.expand(v, a.Vars)
			t, ok := c.Templates[a.template]
			if !ok {
				c.errorf("unknown template %s", a.template)
			}
			a.Template = t
		case "override":
			a.override = c.expand(v, a.Vars)
			o, ok := c.Alerts[a.override]
			if !ok {
				c.errorf("unknown alert %s", a.override)
			}
			a.Overrides = o
			o.Overriders = append(o.Overriders, &a)
		case "crit":
			a.crit = c.expand(v, a.Vars)
			crit, err := expr.New(a.crit)
			if err != nil {
				c.error(err)
			}
			a.Crit = crit
		case "warn":
			a.warn = c.expand(v, a.Vars)
			warn, err := expr.New(a.warn)
			if err != nil {
				c.error(err)
			}
			a.Warn = warn
		default:
			if !strings.HasPrefix(k, "$") {
				c.errorf("unknown key %s", k)
			}
			a.Vars[k] = c.expand(v, a.Vars)
		}
	}
	c.at(s)
	if a.Crit == nil && a.Warn == nil {
		c.errorf("neither crit or warn specified")
	}
	c.Alerts[name] = &a
}

var exRE = regexp.MustCompile(`\$\w+`)

func (c *Conf) expand(v string, vars map[string]string) string {
	v = exRE.ReplaceAllStringFunc(v, func(s string) string {
		if vars != nil {
			if n, ok := vars[s]; ok {
				return n
			}
		}
		n, ok := c.Vars[s]
		if !ok {
			c.errorf("unknown variable %s", s)
		}
		return n
	})
	return v
}
