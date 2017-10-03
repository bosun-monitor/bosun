package rule

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/conf/rule/parse"
	"bosun.org/cmd/bosun/conf/template"
	eparse "bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
)

func (c *Conf) loadTemplate(s *parse.SectionNode) {
	name := s.Name.Text
	if _, ok := c.Templates[name]; ok {
		c.errorf("duplicate template name: %s", name)
	}
	t := conf.Template{
		Vars:            make(map[string]string),
		Name:            name,
		CustomTemplates: map[string]*template.Template{},
		RawCustoms:      map[string]string{},
	}
	t.Text = s.RawText
	t.Locator = newSectionLocator(s)
	funcs := template.FuncMap{
		"V": func(v string) string {
			return c.Expand(v, t.Vars, false)
		},
	}
	saw := make(map[string]bool)
	inherits := []string{}
	var kvps = map[string]string{}
	for _, p := range s.Nodes.Nodes {
		c.at(p)
		switch p := p.(type) {
		case *parse.PairNode:
			c.seen(p.Key.Text, saw)
			if p.Key.Text == "inherit" {
				inherits = append(inherits, p.Val.Text)
			} else {
				kvps[p.Key.Text] = p.Val.Text
			}
		default:
			c.errorf("unexpected node")
		}
	}
	// expand all inherits first, add to kvps if not present
	for _, i := range inherits {
		other, ok := c.Templates[i]
		if !ok {
			c.errorf("cannot inherit unknown template %s", i)
		}
		if other.RawBody != "" && kvps["body"] == "" {
			kvps["body"] = other.RawBody
		}
		if other.RawSubject != "" && kvps["subject"] == "" {
			kvps["subject"] = other.RawSubject
		}
		for k, v := range other.RawCustoms {
			if kvps[k] == "" {
				kvps[k] = v
			}
		}
	}

	// now process like normal
	for k, v := range kvps {
		switch k {
		case "body":
			t.RawBody = v
			tmpl, err := c.bodies.New(name).Funcs(funcs).Parse(t.RawBody)
			if err != nil {
				c.error(err)
			}
			t.Body = tmpl
		case "subject":
			t.RawSubject = v
			tmpl, err := c.subjects.New(name).Funcs(funcs).Parse(t.RawSubject)
			if err != nil {
				c.error(err)
			}
			t.Subject = tmpl
		case "inherit":
			c.errorf("inherit should have been pruned in first pass")
		default:
			if strings.HasPrefix(k, "$") {
				t.Vars[k] = v
				t.Vars[k[1:]] = t.Vars[k]
				continue
			}
			t.RawCustoms[k] = v
			ct, ok := c.customTemplates[k]
			if !ok {
				ct = template.New(k).Funcs(defaultFuncs)
				c.customTemplates[k] = ct
			}
			tmpl, err := ct.New(name).Funcs(funcs).Parse(v)
			if err != nil {
				c.error(err)
			}
			t.CustomTemplates[k] = tmpl
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
		Vars:              make(map[string]string),
		Name:              name,
		CritNotification:  new(conf.Notifications),
		WarnNotification:  new(conf.Notifications),
		AlertTemplateKeys: map[string]*template.Template{},
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
	allNots := c.getAllPossibleNotifications(&a)
	if a.Log {
		for _, n := range allNots {
			if n.Next != nil {
				c.errorf("cannot use log with a chained notification")
			}
		}
		if len(allNots) == 0 {
			c.errorf("log specified but no notification")
		}
	}
	if len(allNots) > 0 && a.Template == nil {
		c.errorf("notifications specified but no template")
	}
	if a.Template != nil {
		if a.Body == nil || a.Subject == nil {
			// alert checks for body or subject since some templates might not be directly used in alerts
			c.errorf("alert templates must have body and subject specified")
		}
		// make sure each notification has it's needed template keys present in this alert's template
		// also build lookup of which template keys need to be rendered at alert time, and which do not
		checkNotification := func(not *conf.Notification) {
			checkSingleKey := func(templateKey string, msg string, alertTime bool) {
				if templateKey == "" || templateKey == "body" || templateKey == "subject" {
					return
				}
				if tmpl := a.Template.CustomTemplates[templateKey]; tmpl != nil {
					if alertTime {
						a.AlertTemplateKeys[templateKey] = tmpl
					}
					return
				}
				errmsg := fmt.Sprintf("notification %s uses template key %s in %s, but template %s does not include it", not.Name, "%s", "%s", a.Template.Name)
				c.errorf(errmsg, templateKey, msg)
			}
			checkTplKeys := func(tks *conf.NotificationTemplateKeys, ctx string, alertTime bool) {
				checkSingleKey(tks.BodyTemplate, ctx+" body template", alertTime)
				checkSingleKey(tks.EmailSubjectTemplate, ctx+" email subject", alertTime)
				checkSingleKey(tks.GetTemplate, ctx+" get url", alertTime)
				checkSingleKey(tks.PostTemplate, ctx+" post url", alertTime)
			}
			checkTplKeys(&not.NotificationTemplateKeys, "alert", true)
			checkTplKeys(&not.UnknownTemplateKeys, "unknown", false)
			checkTplKeys(&not.UnknownMultiTemplateKeys, "unknownMulti", false)
			for at, ntk := range not.ActionTemplateKeys {
				key := at.String()
				if at == models.ActionNone {
					key = "default"
				}
				checkTplKeys(ntk, key, false)
			}
		}
		for _, not := range allNots {
			checkNotification(not)
		}
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
		Vars:               make(map[string]string),
		ContentType:        "application/x-www-form-urlencoded",
		Name:               name,
		RunOnActions:       "all",
		ActionTemplateKeys: map[models.ActionType]*conf.NotificationTemplateKeys{},
		GroupActions:       true,
	}
	n.Text = s.RawText
	n.Locator = newSectionLocator(s)
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

		case "bodyTemplate":
			n.BodyTemplate = v
		case "getTemplate":
			n.GetTemplate = v
		case "postTemplate":
			n.PostTemplate = v
		case "emailSubjectTemplate":
			n.EmailSubjectTemplate = v
		case "runOnActions":
			// todo: validate all/true, none/false, or comma seperated action shortNames
			n.RunOnActions = v
		case "groupActions":
			if v == "false" {
				n.GroupActions = false
			} else if v == "true" {
				n.GroupActions = true
			} else {
				c.errorf("invalid boolean value %s", v)
			}
		case "unknownMinGroupSize":
			i, err := strconv.Atoi(v)
			if err != nil {
				c.error(err)
			}
			n.UnknownMinGroupSize = &i
		case "unknownThreshold":
			i, err := strconv.Atoi(v)
			if err != nil {
				c.error(err)
			}
			n.UnknownThreshold = &i
		default:
			// all special template keys are handled in one loop
			// the following formats are possible:
			// action(templateKey)(ActionType})?   //action
			// unknown(TemplateKey)                //unknown
			// unknownMulti(TemplateKey)           //unknown
			var keys *conf.NotificationTemplateKeys
			keyType := k
			if strings.HasPrefix(k, "action") {
				keyType = strings.TrimPrefix(k, "action")
				at := models.ActionNone
				// look for and trim suffix if there
				for s, t := range models.ActionShortNames {
					if strings.HasSuffix(keyType, s) {
						at = t
						keyType = keyType[:len(keyType)-len(s)]
						break
					}
				}
				if n.ActionTemplateKeys[at] == nil {
					n.ActionTemplateKeys[at] = &conf.NotificationTemplateKeys{}
				}
				keys = n.ActionTemplateKeys[at]
			} else if strings.HasPrefix(k, "unknownMulti") {
				keys = &n.UnknownMultiTemplateKeys
				keyType = strings.TrimPrefix(k, "unknownMulti")
			} else if strings.HasPrefix(k, "unknown") {
				keys = &n.UnknownTemplateKeys
				keyType = strings.TrimPrefix(k, "unknown")
			} else {
				c.errorf("unknown key %s", k)
			}
			switch keyType {
			case "Body":
				keys.BodyTemplate = v
			case "Get":
				keys.GetTemplate = v
			case "Post":
				keys.PostTemplate = v
			case "EmailSubject":
				keys.EmailSubjectTemplate = v
			default:
				c.errorf("unknown key %s", k)
			}
			break

		}
	}
	// TODO: make sure get/getTemplate and post/postTemplate are mutually exclusive
	c.at(s)
	if n.Timeout > 0 && n.Next == nil {
		c.errorf("timeout specified without next")
	}
}
