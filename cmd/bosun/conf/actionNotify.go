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
	subject, err := render(tks.EmailSubjectTemplate, defaultActionNotificationSubjectTemplate)
	if err != nil {
		slog.Errorf("rendering action email subject: %s", err)
	}
	body, err := render(tks.BodyTemplate, defaultActionNotificationBodyTemplate)
	if err != nil {
		slog.Errorf("rendering action body: %s", err)
	}
	postURL, getURL := "", ""
	if tks.PostTemplate != "" {
		postURL, err = render(tks.PostTemplate, nil)
		if err != nil {
			slog.Errorf("rendering action post url: %s", err)
		}
	}
	if tks.GetTemplate != "" {
		getURL, err = render(tks.GetTemplate, nil)
		if err != nil {
			slog.Errorf("rendering action get url: %s", err)
		}
	}
	n.NotifyRaw(subject, body, postURL, getURL, c)
}

func (n *Notification) NotifyRaw(subject, body, postURL, getURL string, c SystemConfProvider) {
	if len(n.Email) > 0 {
		go n.SendEmail(subject, body, c, "")
	}
	if postURL == "" && n.Post != nil {
		postURL = n.Post.String()
	}
	if getURL == "" && n.Get != nil {
		getURL = n.Get.String()
	}
	if postURL != "" {
		n.SendHttp("POST", postURL, body, "")
	}
	if getURL != "" {
		n.SendHttp("GET", getURL, "", "")
	}
}
