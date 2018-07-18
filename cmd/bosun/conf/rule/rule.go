package rule

import (
	"encoding/json"
	"fmt"
	htemplate "html/template"
	"io/ioutil"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"bosun.org/models"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/conf/rule/parse"
	"bosun.org/cmd/bosun/conf/template"
	"bosun.org/cmd/bosun/expr"
	eparse "bosun.org/cmd/bosun/expr/parse"
	"bosun.org/opentsdb"
	"github.com/MiniProfiler/go/miniprofiler"
)

type Conf struct {
	Vars conf.Vars
	Name string // Config file name

	Templates     map[string]*conf.Template
	Alerts        map[string]*conf.Alert
	Notifications map[string]*conf.Notification `json:"-"`
	RawText       string
	Macros        map[string]*conf.Macro
	Lookups       map[string]*conf.Lookup
	Squelch       conf.Squelches `json:"-"`
	NoSleep       bool

	reload   func() error
	backends conf.EnabledBackends

	sysVars map[string]string

	tree            *parse.Tree
	node            parse.Node
	unknownTemplate string
	bodies          *template.Template
	subjects        *template.Template
	customTemplates map[string]*template.Template
	squelch         []string

	writeLock chan bool

	deferredSections map[string][]deferredSection // SectionType:[]deferredSection
	saveHook         conf.SaveHook                // func that gets called on save if not nil
	Hash             string
}

type deferredSection struct {
	LoadFunc    func(*parse.SectionNode)
	SectionNode *parse.SectionNode
}

func (c *Conf) AlertSquelched(a *conf.Alert) func(opentsdb.TagSet) bool {
	return func(tags opentsdb.TagSet) bool {
		return c.Squelched(a, tags)
	}
}

func (c *Conf) Squelched(a *conf.Alert, tags opentsdb.TagSet) bool {
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

// parseNotifications parses the comma-separated string v for notifications and
// returns them.
func (c *Conf) parseNotifications(v string) (map[string]*conf.Notification, error) {
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

func ParseFile(fname string, backends conf.EnabledBackends, sysVars map[string]string) (*Conf, error) {
	f, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	return NewConf(fname, backends, sysVars, string(f))
}

func (c *Conf) SaveConf(newConf *Conf) error {
	return ioutil.WriteFile(c.Name, []byte(newConf.RawText), os.FileMode(int(0640)))
}

func NewConf(name string, backends conf.EnabledBackends, sysVars map[string]string, text string) (c *Conf, err error) {
	defer errRecover(&err)
	c = &Conf{
		Name:             name,
		Vars:             make(map[string]string),
		Templates:        make(map[string]*conf.Template),
		Alerts:           make(map[string]*conf.Alert),
		Notifications:    make(map[string]*conf.Notification),
		RawText:          text,
		bodies:           template.New("body").Funcs(defaultFuncs),
		subjects:         template.New("subject").Funcs(defaultFuncs),
		customTemplates:  map[string]*template.Template{},
		Lookups:          make(map[string]*conf.Lookup),
		Macros:           make(map[string]*conf.Macro),
		writeLock:        make(chan bool, 1),
		deferredSections: make(map[string][]deferredSection),
		backends:         backends,
		sysVars:          sysVars,
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

	loadSections := func(sectionType string) {
		for _, dSec := range c.deferredSections[sectionType] {
			c.at(dSec.SectionNode)
			dSec.LoadFunc(dSec.SectionNode)
		}
	}

	loadSections("template")
	loadSections("macro")
	loadSections("notification")
	loadSections("lookup")
	loadSections("alert")

	c.genHash()
	return
}

func (c *Conf) loadGlobal(p *parse.PairNode) {
	v := c.Expand(p.Val.Text, nil, false)
	switch k := p.Key.Text; k {
	case "unknownTemplate":
		c.unknownTemplate = v
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
	ds := deferredSection{}
	switch s.SectionType.Text {
	case "template":
		ds.LoadFunc = c.loadTemplate
	case "alert":
		ds.LoadFunc = c.loadAlert
	case "notification":
		ds.LoadFunc = c.loadNotification
	case "macro":
		ds.LoadFunc = c.loadMacro
	case "lookup":
		ds.LoadFunc = c.loadLookup
	default:
		c.errorf("unknown section type: %s", s.SectionType.Text)
	}
	ds.SectionNode = s
	c.deferredSections[s.SectionType.Text] = append(c.deferredSections[s.SectionType.Text], ds)
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

func (c *Conf) getPairs(s *parse.SectionNode, vars conf.Vars, st sectionType) (pairs []nodePair) {
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

func (c *Conf) loadLookup(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Lookups[name]; ok {
		c.errorf("duplicate lookup name: %s", name)
	}
	l := conf.Lookup{
		Name: name,
	}
	l.Text = s.RawText
	l.Locator = newSectionLocator(s)
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

func (c *Conf) loadMacro(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Macros[name]; ok {
		c.errorf("duplicate macro name: %s", name)
	}
	m := conf.Macro{
		Name: name,
	}
	m.Text = s.RawText
	m.Locator = newSectionLocator(s)
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

// Note: Funcs that can error should return a pointer. In the error case the pointer
// should be non-nil. The exception to this is when a string is returned, in which case
// the string format of the error should be returned. This allows for error handling within
// templates for information that is helpful but not stricly necessary
var defaultFuncs = template.FuncMap{
	"bytes": func(v interface{}) string {
		switch v := v.(type) {
		case string:
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return err.Error()
			}
			return conf.ByteSize(f).String()
		case int:
			return conf.ByteSize(v).String()
		case float64:
			return conf.ByteSize(v).String()
		case expr.Number:
			return conf.ByteSize(v).String()
		case expr.Scalar:
			return conf.ByteSize(v).String()
		}
		return fmt.Errorf("unexpected type passed to bytes function: %T (%v)", v, v).Error()
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
	// There is a trap here, this will only make sure that
	// it is a untyped nil. A typed nil would return true
	// here. So it is vital that we only return literal
	// nils from functions when they error with notNil.
	// This is needed because template's conditionals
	// treat things like 0 and false as "not true" just like
	// nil.
	"notNil": func(value interface{}) bool {
		if value == nil {
			return false
		}
		return true
	},
	"parseDuration": func(s string) *time.Duration {
		d, err := time.ParseDuration(s)
		if err != nil {
			return nil
		}
		return &d
	},
	"makeSlice": func(vals ...interface{}) interface{} {
		return vals
	},
	"makeMap": func(vals ...interface{}) interface{} {
		if len(vals)%2 != 0 {
			return fmt.Errorf("MakeMap requires even number of arguments").Error()
		}
		m := map[string]interface{}{}
		for i := 0; i < len(vals); i += 2 {
			key, ok := vals[i].(string)
			if !ok {
				return fmt.Errorf("MakeMap requires all map keys to be strings").Error()
			}
			m[key] = vals[i+1]
		}
		return m
	},
	"json": func(v interface{}) string {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err.Error()
		}
		return string(b)
	},
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
		} else if strings.HasPrefix(s, "$sys.") {
			n = c.sysVars[s[5:]]
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

func (c *Conf) NewExpr(s string) *expr.Expr {
	exp, err := expr.New(s, c.GetFuncs(c.backends))
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

func (c *Conf) GetFuncs(backends conf.EnabledBackends) map[string]eparse.Func {
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
	if backends.OpenTSDB {
		merge(expr.TSDB)
	}
	if backends.Graphite {
		merge(expr.Graphite)
	}
	if backends.Elastic {
		merge(expr.Elastic)
	}
	if backends.Influx {
		merge(expr.Influx)
	}
	if backends.Annotate {
		merge(expr.Annotate)
	}
	if backends.AzureMonitor {
		merge(expr.AzureMonitor)
	}
	return funcs
}

func (c *Conf) getAlertExpr(name, key string) (*conf.Alert, *expr.Expr, error) {
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

func (c *Conf) GetTemplate(s string) *conf.Template {
	return c.Templates[s]
}

func (c *Conf) GetAlerts() map[string]*conf.Alert {
	return c.Alerts
}

func (c *Conf) GetAlert(s string) *conf.Alert {
	return c.Alerts[s]
}

func (c *Conf) GetNotifications() map[string]*conf.Notification {
	return c.Notifications
}

func (c *Conf) GetNotification(s string) *conf.Notification {
	return c.Notifications[s]
}

func (c *Conf) GetMacro(s string) *conf.Macro {
	return c.Macros[s]
}

func (c *Conf) GetLookup(s string) *conf.Lookup {
	return c.Lookups[s]
}

func (c *Conf) GetSquelches() conf.Squelches {
	return c.Squelch
}

func (c *Conf) GetRawText() string {
	return c.RawText
}

func (c *Conf) SetReload(reload func() error) {
	c.reload = reload
}

func (c *Conf) Reload() error {
	return c.reload()
}

func (c *Conf) SetSaveHook(sh conf.SaveHook) {
	c.saveHook = sh
}

func (c *Conf) callSaveHook(file, user, message string, args ...string) error {
	if c.saveHook == nil {
		return nil
	}
	return c.saveHook(file, user, message, args...)
}

func (c *Conf) genHash() {
	c.Hash = conf.GenHash(c.RawText)
}

func (c *Conf) GetHash() string {
	return c.Hash
}

// returns any notifications accessible from the alert vis warn/critNotification, including chains and lookups
func (c *Conf) getAllPossibleNotifications(a *conf.Alert) map[string]*conf.Notification {
	nots := map[string]*conf.Notification{}
	for k, v := range a.WarnNotification.GetAllChained() {
		nots[k] = v
	}
	for k, v := range a.CritNotification.GetAllChained() {
		nots[k] = v
	}
	followLookup := func(l map[string]*conf.Lookup) {
		for target, lookup := range l {
			for _, entry := range lookup.Entries {
				if notNames, ok := entry.Values[target]; ok {
					for _, k := range strings.Split(notNames, ",") {
						if not, ok := c.Notifications[k]; ok {
							nots[k] = not
						} else {
							c.errorf("Notification %s needed by lookup %s in %s is not defined.", k, lookup.Name, a.Name)
						}
					}
				}
			}
		}
	}
	followLookup(a.CritNotification.Lookups)
	followLookup(a.WarnNotification.Lookups)
	return nots
}
