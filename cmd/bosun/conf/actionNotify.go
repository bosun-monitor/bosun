package conf

import (
	"bytes"
	"strings"

	"bosun.org/slog"

	"bosun.org/cmd/bosun/conf/template"
	"bosun.org/models"
)

var defaultActionNotificationSubjectTemplate *template.Template
var defaultActionNotificationBodyTemplate *template.Template

func init() {
	subject := `{{$first := index .States 0}}{{$count := len .States}}
{{.User}} {{.ActionType}}
{{if gt $count 1}} {{$count}} Alerts. 
{{else}} Incident #{{$first.Id}} ({{$first.Subject}}) 
{{end}}`
	body := `{{$count := len .States}}{{.User}} {{.ActionType}} {{$count}} alert{{if gt $count 1}}s{{end}}: <br/>
<strong>Message:</strong> {{.Message}} <br/>
<strong>Incidents:</strong> <br/>
<ul>
	{{range .States}}
		<li>
			<a href="{{$.IncidentLink .Id}}">#{{.Id}}:</a> 
			{{.Subject}}
		</li>
	{{end}}
</ul>`
	defaultActionNotificationSubjectTemplate = template.Must(template.New("subject").Parse(strings.Replace(subject, "\n", "", -1)))
	defaultActionNotificationBodyTemplate = template.Must(template.New("body").Parse(body))
}

// NotifyAction should be used for action notifications.
// ctx should be sched.actionNotificationContext
func (n *Notification) NotifyAction(at models.ActionType, t *Template, c SystemConfProvider, ctx interface{}) {
	n.PrepareAction(at, t, ctx).Send(c)
}

func (n *Notification) PrepareAction(at models.ActionType, t *Template, ctx interface{}) *PreparedNotifications {
	pn := &PreparedNotifications{}
	// get template keys to use for things. Merge with default sets
	tks := n.ActionTemplateKeys[at].Combine(n.ActionTemplateKeys[models.ActionNone])
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
			slog.Errorf("executing action template '%s': %s", key, err)
			return "", err
		}
		return buf.String(), nil
	}
	renderSubject := func(key string) (string, error) { return render(key, defaultActionNotificationSubjectTemplate) }
	renderBody := func(key string) (string, error) { return render(key, defaultActionNotificationBodyTemplate) }

	var body string
	var err error
	if len(n.Email) > 0 || n.Post != nil || tks.PostTemplate != "" {
		body, err = renderBody(tks.BodyTemplate)
		if err != nil {
			slog.Errorf("rendering action body: %s", err)
		}
	}

	if len(n.Email) > 0 {
		subject, err := renderSubject(tks.EmailSubjectTemplate)
		if err != nil {
			slog.Errorf("rendering action email subject: %s", err)
		} else {
			pn.Email = n.PrepEmail(subject, body, "", nil)
		}
	}

	postURL, getURL := "", ""
	if tks.PostTemplate != "" {
		postURL, err = render(tks.PostTemplate, nil)
		if err != nil {
			slog.Errorf("rendering action post url: %s", err)
		}
	} else if n.Post != nil {
		postURL = n.Post.String()
	}
	if tks.GetTemplate != "" {
		getURL, err = render(tks.GetTemplate, nil)
		if err != nil {
			slog.Errorf("rendering action get url: %s", err)
		}
	} else if n.Get != nil {
		getURL = n.Get.String()
	}
	if postURL != "" {
		pn.HTTP = append(pn.HTTP, n.PrepHttp("POST", postURL, body, ""))
	}
	if getURL != "" {
		pn.HTTP = append(pn.HTTP, n.PrepHttp("GET", getURL, "", ""))
	}
	return pn
}
