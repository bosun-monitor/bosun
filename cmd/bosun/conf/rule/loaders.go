package rule

import (
	"encoding/json"
	htemplate "html/template"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	ttemplate "text/template"
	"time"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/conf/rule/parse"
	eparse "bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

func (c *Conf) loadTemplate(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Templates[name]; ok {
		c.errorf("duplicate template name: %s", name)
	}
	t := conf.Template{
		Vars:            make(map[string]string),
		Name:            name,
		CustomTemplates: map[string]conf.GenericTemplate{},
		RawCustoms:      map[string]string{},
	}
	t.Text = s.RawText
	t.Locator = newSectionLocator(s)
	funcs := ttemplate.FuncMap{
		"V": func(v string) string {
			return c.Expand(v, t.Vars, false)
		},
	}
	hFuncs := htemplate.FuncMap(funcs)
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
				tmpl := c.bodies.New(name).Funcs(hFuncs)
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
				if strings.HasPrefix(k, "$") {
					t.Vars[k] = v
					t.Vars[k[1:]] = t.Vars[k]
					continue
				}
				t.RawCustoms[k] = v
				var ex conf.GenericTemplate
				var err error
				if isHTMLTemplate(k) {
					t, ok := c.customHtmlTemplates[k]
					if !ok {
						t = htemplate.New(c.Name).Funcs(hFuncs)
						c.customHtmlTemplates[k] = t
					}
					ex, err = t.New(name).Funcs(hFuncs).Parse(v)
				} else {
					t, ok := c.customTextTemplates[k]
					if !ok {
						t = ttemplate.New(c.Name).Funcs(funcs)
						c.customTextTemplates[k] = t
					}
					ex, err = t.New(name).Funcs(funcs).Parse(v)
				}
				if err != nil {
					c.error(err)
				}
				t.CustomTemplates[k] = ex
			}
		default:
			c.errorf("unexpected node")
		}
	}
	c.at(s)
	c.Templates[name] = &t
}

func isHTMLTemplate(name string) bool {
	name = strings.ToLower(name)
	if name == "emailbody" || strings.HasSuffix(name, "html") {
		return true
	}
	return false
}

var lookupNotificationRE = regexp.MustCompile(`^lookup\("(.*)", "(.*)"\)$`)

func (c *Conf) loadAlert(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Alerts[name]; ok {
		c.errorf("duplicate alert name: %s", name)
	}
	a := conf.Alert{
		Vars:             make(map[string]string),
		Name:             name,
		CritNotification: new(conf.Notifications),
		WarnNotification: new(conf.Notifications),
	}
	a.Text = s.RawText
	a.Locator = newSectionLocator(s)
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
		if len(depTags) != 0 && len(depTags.Intersection(tags)) < 1 {
			c.errorf("Depends and crit/warn must share at least one tag.")
		}
	}
	warnLength := len(a.WarnNotification.Notifications) + len(a.WarnNotification.Lookups)
	critLength := len(a.CritNotification.Notifications) + len(a.CritNotification.Lookups)
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
		if warnLength+critLength == 0 {
			c.errorf("log specified but no notification")
		}
	}
	if warnLength+critLength > 0 && a.Template == nil {
		c.errorf("notifications specified but no template")
	}
	if a.Template != nil && a.Body == nil && a.Subject == nil {
		// alert checks for body or subject since some templates might not be directly used in alerts
		c.errorf("alert templates must have body or subject specified")
	}
	a.ReturnType = ret
	c.Alerts[name] = &a
}

func (c *Conf) loadNotification(s *parse.SectionNode) {
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
	n.Locator = newSectionLocator(s)
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
