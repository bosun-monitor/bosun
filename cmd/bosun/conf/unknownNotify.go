package conf

import (
	"bytes"
	"fmt"
	"time"

	"bosun.org/cmd/bosun/conf/template"
	"bosun.org/models"
	"bosun.org/slog"
)

type unknownContext struct {
	Time  time.Time
	Name  string
	Group models.AlertKeys
}

var defaultUnknownTemplate = &Template{
	Body: template.Must(template.New("body").Parse(`
		<p>Time: {{.Time}}
		<p>Name: {{.Name}}
		<p>Alerts:
		{{range .Group}}
			<br>{{.}}
		{{end}}
	`)),
	Subject: template.Must(template.New("subject").Parse(`{{.Name}}: {{.Group | len}} unknown alerts`)),
}

var unknownDefaults defaultTemplates

func init() {
	subject := `{{.Name}}: {{.Group | len}} unknown alerts`
	body := `
	<p>Time: {{.Time}}
	<p>Name: {{.Name}}
	<p>Alerts:
	{{range .Group}}
		<br>{{.}}
	{{end}}
`
	unknownDefaults.subject = template.Must(template.New("subject").Parse(subject))
	unknownDefaults.body = template.Must(template.New("body").Parse(body))
}

func (n *Notification) PrepareUnknown(t *Template, c SystemConfProvider, name string, aks []models.AlertKey) *PreparedNotifications {
	ctx := &unknownContext{
		Time:  time.Now().UTC(),
		Name:  name,
		Group: aks,
	}
	pn := &PreparedNotifications{}
	buf := &bytes.Buffer{}
	render := func(key string, defaultTmpl *template.Template) (string, error) {
		tpl := defaultTmpl
		if key != "" {
			tpl = t.Get(key)
		} else {
			key = "default"
		}
		buf.Reset()
		err := tpl.Execute(buf, ctx)
		if err != nil {
			e := fmt.Sprintf("executing unknown template '%s': %s", key, err)
			pn.Errors = append(pn.Errors, e)
			slog.Errorf(e)
			return "", err
		}
		return buf.String(), nil
	}

	tks := n.UnknownTemplateKeys

	ak := map[string][]string{
		"alert_key": {},
	}

	for i := range aks {
		contain := func(key string, array []string) bool {
			for _, value := range array {
				if value == key {
					return true
				}
			}
			return false
		}
		if contain(fmt.Sprint(aks[i]), ak["alert_key"]) != true {
			ak["alert_key"] = append(ak["alert_key"], fmt.Sprint(aks[i]))
		}
	}

	details := &NotificationDetails{
		NotifyName:  n.Name,
		TemplateKey: tks.BodyTemplate,
		Ak:          ak["alert_key"],
		NotifyType:  2,
	}

	n.prepareFromTemplateKeys(pn, tks, render, unknownDefaults, details)
	return pn
}

func (n *Notification) NotifyUnknown(t *Template, c SystemConfProvider, name string, aks []models.AlertKey) {
	go n.PrepareUnknown(t, c, name, aks).Send(c)
}

var unknownMultiDefaults defaultTemplates

type unknownMultiContext struct {
	Time      time.Time
	Threshold int
	Groups    map[string]models.AlertKeys
}

func init() {
	subject := `{{.Groups | len}} unknown alert instances suppressed`
	body := `
	<p>Threshold of {{ .Threshold }} reached for unknown notifications. The following unknown
	group emails were not sent.
	<ul>
	{{ range $group, $alertKeys := .Groups }}
		<li>
			{{ $group }}
			<ul>
				{{ range $ak := $alertKeys }}
				<li>{{ $ak }}</li>
				{{ end }}
			</ul>
		</li>
	{{ end }}
	</ul>
	`
	unknownMultiDefaults.subject = template.Must(template.New("subject").Parse(subject))
	unknownMultiDefaults.body = template.Must(template.New("body").Parse(body))
}

func (n *Notification) PrepareMultipleUnknowns(t *Template, c SystemConfProvider, groups map[string]models.AlertKeys) *PreparedNotifications {
	ctx := &unknownMultiContext{
		Time:      time.Now().UTC(),
		Threshold: c.GetUnknownThreshold(),
		Groups:    groups,
	}
	pn := &PreparedNotifications{}
	buf := &bytes.Buffer{}
	render := func(key string, defaultTmpl *template.Template) (string, error) {
		tpl := defaultTmpl
		if key != "" {
			tpl = t.Get(key)
		} else {
			key = "default"
		}
		buf.Reset()
		err := tpl.Execute(buf, ctx)
		if err != nil {
			e := fmt.Sprintf("executing unknown multi template '%s': %s", key, err)
			pn.Errors = append(pn.Errors, e)
			slog.Errorf(e)
			return "", err
		}
		return buf.String(), nil
	}

	tks := n.UnknownMultiTemplateKeys

	ak := []string{}

	for _, v := range groups {
		ak = append(ak, fmt.Sprint(v))
	}

	details := &NotificationDetails{
		NotifyName:  n.Name,
		TemplateKey: tks.BodyTemplate,
		Ak:          ak,
		NotifyType:  3,
	}

	n.prepareFromTemplateKeys(pn, tks, render, unknownMultiDefaults, details)
	return pn
}

func (n *Notification) NotifyMultipleUnknowns(t *Template, c SystemConfProvider, groups map[string]models.AlertKeys) {
	n.PrepareMultipleUnknowns(t, c, groups).Send(c)
}

// code common to PrepareAction / PrepareUnknown / PrepareMultipleUnknowns
func (n *Notification) prepareFromTemplateKeys(pn *PreparedNotifications, tks NotificationTemplateKeys, render func(string, *template.Template) (string, error), defaults defaultTemplates, alertDetails *NotificationDetails) {

	if len(n.Email) > 0 || n.Post != nil || tks.PostTemplate != "" {
		body, _ := render(tks.BodyTemplate, defaults.body)
		if subject, err := render(tks.EmailSubjectTemplate, defaults.subject); err == nil {
			pn.Email = n.PrepEmail(subject, body, "", nil)
		}
	}

	postURL, getURL := "", ""
	if tks.PostTemplate != "" {
		if p, err := render(tks.PostTemplate, nil); err == nil {
			postURL = p
		}
	} else if n.Post != nil {
		postURL = n.Post.String()
	}
	if tks.GetTemplate != "" {
		if g, err := render(tks.GetTemplate, nil); err == nil {
			getURL = g
		}
	} else if n.Get != nil {
		getURL = n.Get.String()
	}
	if postURL != "" {
		body, _ := render(tks.BodyTemplate, defaults.subject)
		pn.HTTP = append(pn.HTTP, n.PrepHttp("POST", postURL, body, alertDetails))
	}
	if getURL != "" {
		pn.HTTP = append(pn.HTTP, n.PrepHttp("GET", getURL, "", alertDetails))
	}
}
