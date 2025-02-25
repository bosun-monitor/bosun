package conf

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/mail"
	"net/smtp"
	"strings"

	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/models"
	"bosun.org/slog"
	"bosun.org/util"
	"github.com/jordan-wright/email"
)

const (
	sendLogSuccessFmt = "%s; name: %s; transport: %s; dst: %s; body: %s"
	sendLogErrorFmt   = "%s; name: %s; transport: %s; dst: %s; body: %s; error: %s"
	httpSendErrorFmt  = "bad response for '%s' %s notification using template key '%s' for alert keys %v method %s: %d"
)

func init() {
	metadata.AddMetricMeta(
		"bosun.email.sent", metadata.Counter, metadata.PerSecond,
		"The number of email notifications sent by Bosun.")
	metadata.AddMetricMeta(
		"bosun.email.sent_failed", metadata.Counter, metadata.PerSecond,
		"The number of email notifications that Bosun failed to send.")
	metadata.AddMetricMeta(
		"bosun.post.sent", metadata.Counter, metadata.PerSecond,
		"The number of post notifications sent by Bosun.")
	metadata.AddMetricMeta(
		"bosun.post.sent_failed", metadata.Counter, metadata.PerSecond,
		"The number of post notifications that Bosun failed to send.")
}

type PreparedNotifications struct {
	Email  *PreparedEmail
	HTTP   []*PreparedHttp
	Print  bool
	Name   string
	Errors []string
}

func (p *PreparedNotifications) Send(c SystemConfProvider) (errs []error) {
	if p.Email != nil {
		if err := p.Email.Send(c); err != nil {
			slog.Errorf(
				sendLogErrorFmt,
				fmt.Sprintf("subject: %s", p.Email.Subject),
				p.Name,
				"email",
				strings.Join(p.Email.To, ","),
				p.Email.Body,
				err.Error(),
			)
			errs = append(errs, err)
		} else if p.Print {
			slog.Infof(
				sendLogSuccessFmt,
				fmt.Sprintf("subject: %s", p.Email.Subject),
				p.Name,
				"email",
				strings.Join(p.Email.To, ","),
				p.Email.Body,
			)
		}
	}
	for _, h := range p.HTTP {
		var logPrefix string
		if h.Details.At != "" {
			logPrefix = fmt.Sprintf("action_type: %s", h.Details.At)
		} else {
			logPrefix = "type: alert"
		}
		if _, err := h.Send(); err != nil {
			slog.Errorf(
				sendLogErrorFmt,
				logPrefix,
				h.Details.NotifyName,
				"http_"+h.Method,
				h.URL,
				h.Body,
				err.Error(),
			)
			errs = append(errs, err)
		} else if p.Print {
			slog.Infof(
				sendLogSuccessFmt,
				logPrefix,
				h.Details.NotifyName,
				"http_"+h.Method,
				h.URL,
				h.Body,
			)
		}
	}

	return
}

// PrepareAlert does all of the work of selecting what content to send to which sources. It does not actually send any notifications,
// but the returned object can be used to send them.
func (n *Notification) PrepareAlert(rt *models.RenderedTemplates, ak string, attachments ...*models.Attachment) *PreparedNotifications {
	pn := &PreparedNotifications{Name: n.Name, Print: n.Print}
	if len(n.Email) > 0 {
		subject := rt.GetDefault(n.EmailSubjectTemplate, "emailSubject")
		body := rt.GetDefault(n.BodyTemplate, "emailBody")
		pn.Email = n.PrepEmail(subject, body, ak, attachments)
	}
	if n.Post != nil || n.PostTemplate != "" {
		url := ""
		if n.Post != nil {
			url = n.Post.String()
		} else {
			url = rt.Get(n.PostTemplate)
		}
		body := rt.GetDefault(n.BodyTemplate, "subject")
		details := &NotificationDetails{
			Ak:          []string{ak},
			NotifyName:  n.Name,
			TemplateKey: n.BodyTemplate,
			NotifyType:  1,
		}
		pn.HTTP = append(pn.HTTP, n.PrepHttp("POST", url, body, details))
	}
	if n.Get != nil || n.GetTemplate != "" {
		url := ""
		if n.Get != nil {
			url = n.Get.String()
		} else {
			url = rt.Get(n.GetTemplate)
		}
		details := &NotificationDetails{
			Ak:          []string{ak},
			NotifyName:  n.Name,
			TemplateKey: n.BodyTemplate,
			NotifyType:  1,
		}
		pn.HTTP = append(pn.HTTP, n.PrepHttp("GET", url, "", details))
	}
	return pn
}

// NotifyAlert triggers Email/HTTP/Print actions for the Notification object. Called when an alert is first triggered, or on escalations.
func (n *Notification) NotifyAlert(rt *models.RenderedTemplates, c SystemConfProvider, ak string, attachments ...*models.Attachment) {
	go n.PrepareAlert(rt, ak, attachments...).Send(c)
}

type PreparedHttp struct {
	URL     string
	Method  string
	Headers map[string]string `json:",omitempty"`
	Body    string
	Details *NotificationDetails
}

const (
	alert = iota + 1
	unknown
	multiunknown
)

type NotificationDetails struct {
	Ak          []string // alert key
	At          string   // action type
	NotifyName  string   // notification name
	TemplateKey string   // template key
	NotifyType  int      // notifications type e.g alert, unknown etc
}

func (p *PreparedHttp) Send() (int, error) {
	var body io.Reader
	if p.Body != "" {
		body = strings.NewReader(p.Body)
	}
	req, err := http.NewRequest(p.Method, p.URL, body)
	if err != nil {
		return 0, err
	}
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if resp != nil && resp.Body != nil {
		// Drain up to 512 bytes and close the body to let the Transport reuse the connection
		io.CopyN(ioutil.Discard, resp.Body, 512)
		resp.Body.Close()
	}
	if err != nil {
		return 0, err
	}
	if resp.StatusCode >= 300 {
		collect.Add("post.sent_failed", nil, 1)
		switch p.Details.NotifyType {
		case alert:
			return resp.StatusCode, fmt.Errorf(
				httpSendErrorFmt,
				p.Details.NotifyName,
				"alert",
				p.Details.TemplateKey,
				strings.Join(p.Details.Ak, ","),
				p.Method,
				resp.StatusCode,
			)
		case unknown:
			return resp.StatusCode, fmt.Errorf(
				httpSendErrorFmt,
				p.Details.NotifyName,
				"unknown",
				p.Details.TemplateKey,
				strings.Join(p.Details.Ak, ","),
				p.Method,
				resp.StatusCode,
			)
		case multiunknown:
			return resp.StatusCode, fmt.Errorf(
				httpSendErrorFmt,
				p.Details.NotifyName,
				"multi-unknown",
				p.Details.TemplateKey,
				strings.Join(p.Details.Ak, ","),
				p.Method,
				resp.StatusCode,
			)
		default:
			return resp.StatusCode, fmt.Errorf(
				httpSendErrorFmt,
				p.Details.NotifyName,
				fmt.Sprintf("action '%s'", p.Details.At),
				p.Details.TemplateKey,
				strings.Join(p.Details.Ak, ","),
				p.Method,
				resp.StatusCode,
			)
		}
	}
	collect.Add("post.sent", nil, 1)
	return resp.StatusCode, nil
}

func (n *Notification) PrepHttp(method string, url string, body string, alertDetails *NotificationDetails) *PreparedHttp {
	prep := &PreparedHttp{
		Method:  method,
		URL:     url,
		Headers: map[string]string{},
		Details: alertDetails,
	}
	if method == http.MethodPost {
		prep.Body = body
		prep.Headers["Content-Type"] = n.ContentType
	}
	return prep
}

func (n *Notification) SendHttp(method string, url string, body string) {
	details := &NotificationDetails{}
	p := n.PrepHttp(method, url, body, details)
	stat, err := p.Send()
	if err != nil {
		slog.Errorf("Sending http notification: %s", err)
	}
	slog.Infof("%s notification successful for alert %s. Status: %d", method, details.Ak, stat)
}

type PreparedEmail struct {
	To          []string
	Subject     string
	Body        string
	AK          string
	Attachments []*models.Attachment
}

func (n *Notification) PrepEmail(subject, body string, ak string, attachments []*models.Attachment) *PreparedEmail {
	pe := &PreparedEmail{
		Subject:     subject,
		Body:        body,
		Attachments: attachments,
		AK:          ak,
	}
	for _, a := range n.Email {
		pe.To = append(pe.To, a.Address)
	}
	return pe
}

func (p *PreparedEmail) Send(c SystemConfProvider) error {
	// make sure "To" was not null
	if len(p.To) <= 0 {
		return nil
	}

	e := email.NewEmail()
	e.From = c.GetEmailFrom()
	e.To = append(e.To, p.To...)
	e.Subject = p.Subject
	e.HTML = []byte(p.Body)
	for _, a := range p.Attachments {
		e.Attach(bytes.NewBuffer(a.Data), a.Filename, a.ContentType)
	}
	e.Headers.Add("X-Bosun-Server", util.GetHostManager().GetHostName())
	if err := sendEmail(e, c.GetSMTPHost(), c.GetSMTPUsername(), c.GetSMTPPassword()); err != nil {
		collect.Add("email.sent_failed", nil, 1)
		slog.Errorf("failed to send alert %v to %v %v\n", p.AK, e.To, err)
		return err
	}
	collect.Add("email.sent", nil, 1)
	slog.Infof("relayed email %v to %v sucessfully. Subject: %d bytes. Body: %d bytes.", p.AK, e.To, len(e.Subject), len(e.HTML))
	return nil
}

// Send an email using the given host and SMTP auth (optional), returns any
// error thrown by smtp.SendMail. This function merges the To, Cc, and Bcc
// fields and calls the smtp.SendMail function using the Email.Bytes() output as
// the message.
func sendEmail(e *email.Email, addr, username, password string) error {
	// Merge the To, Cc, and Bcc fields
	to := make([]string, 0, len(e.To)+len(e.Cc)+len(e.Bcc))
	to = append(append(append(to, e.To...), e.Cc...), e.Bcc...)
	// Check to make sure there is at least one recipient and one "From" address
	if e.From == "" || len(to) == 0 {
		return errors.New("Must specify at least one From address and one To address")
	}
	from, err := mail.ParseAddress(e.From)
	if err != nil {
		return err
	}
	raw, err := e.Bytes()
	if err != nil {
		return err
	}
	return smtpSend(addr, username, password, from.Address, to, raw)
}

// SendMail connects to the server at addr, switches to TLS if
// possible, authenticates with the optional mechanism a if possible,
// and then sends an email from address from, to addresses to, with
// message msg.
func smtpSend(addr, username, password string, from string, to []string, msg []byte) error {
	c, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer c.Close()
	if err = c.Hello("localhost"); err != nil {
		return err
	}
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err = c.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil {
			return err
		}
		if len(username) > 0 || len(password) > 0 {
			hostWithoutPort := strings.Split(addr, ":")[0]
			auth := smtp.PlainAuth("", username, password, hostWithoutPort)
			if err = c.Auth(auth); err != nil {
				return err
			}
		}
	}
	if err = c.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err = c.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msg)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	return c.Quit()
}
