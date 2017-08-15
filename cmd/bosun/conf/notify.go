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

func init() {
	metadata.AddMetricMeta(
		"bosun.email.sent", metadata.Counter, metadata.PerSecond,
		"The number of email notifications sent by Bosun.")
	metadata.AddMetricMeta(
		"bosun.email.sent_failed", metadata.Counter, metadata.PerSecond,
		"The number of email notifications that Bosun failed to send.")
}

// NotifyAlert triggers Email/HTTP/Print actions for the Notification object. Called when an alert is first triggered, or on escalations.
func (n *Notification) NotifyAlert(rt *models.RenderedTemplates, c SystemConfProvider, ak string, attachments ...*models.Attachment) {
	if len(n.Email) > 0 {
		subject := rt.GetDefault(n.EmailSubjectTemplate, "emailSubject")
		body := rt.GetDefault(n.BodyTemplate, "emailBody")
		go n.SendEmail(subject, body, c, ak, attachments...)
	}
	if n.Post != nil || n.PostTemplate != "" {
		url := ""
		if n.Post != nil {
			url = n.Post.String()
		} else {
			url = rt.Get(n.PostTemplate)
		}
		body := rt.GetDefault(n.BodyTemplate, "subject")
		n.SendHttp("POST", url, body, ak)
	}
	if n.Get != nil || n.GetTemplate != "" {
		url := ""
		if n.Get != nil {
			url = n.Get.String()
		} else {
			url = rt.Get(n.GetTemplate)
		}
		n.SendHttp("GET", url, "", ak)
	}
	if n.Print {
		if n.BodyTemplate != "" {
			go n.DoPrint("Subject: " + rt.Subject + ", Body: " + rt.Get(n.BodyTemplate))
		} else {
			go n.DoPrint(rt.Subject)
		}
	}
}

func (n *Notification) DoPrint(payload string) {
	slog.Infoln(payload)
}

type PreparedHttp struct {
	URL     string
	Method  string
	Headers map[string]string `json:",omitempty"`
	Body    string
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
		return resp.StatusCode, fmt.Errorf("bad response on notification %s: %d", p.Method, resp.StatusCode)
	}
	return resp.StatusCode, nil
}

func (n *Notification) PrepHttp(method string, url string, body string, ak string) *PreparedHttp {
	prep := &PreparedHttp{
		Method:  method,
		URL:     url,
		Headers: map[string]string{},
	}
	if method == http.MethodPost {
		prep.Body = body
		prep.Headers["Content-Type"] = n.ContentType
	}
	return prep
}

func (n *Notification) SendHttp(method string, url string, body string, ak string) {
	p := n.PrepHttp(method, url, body, ak)
	stat, err := p.Send()
	if err != nil {
		slog.Errorf("Sending http notification: %s", err)
	}
	slog.Infof("%s notification successful for alert %s. Status: %d", method, ak, stat)
}

func (n *Notification) SendEmail(subject, body string, c SystemConfProvider, ak string, attachments ...*models.Attachment) {
	e := email.NewEmail()
	e.From = c.GetEmailFrom()
	for _, a := range n.Email {
		e.To = append(e.To, a.Address)
	}
	e.Subject = subject
	e.HTML = []byte(body)
	for _, a := range attachments {
		e.Attach(bytes.NewBuffer(a.Data), a.Filename, a.ContentType)
	}
	e.Headers.Add("X-Bosun-Server", util.Hostname)
	if err := Send(e, c.GetSMTPHost(), c.GetSMTPUsername(), c.GetSMTPPassword()); err != nil {
		collect.Add("email.sent_failed", nil, 1)
		slog.Errorf("failed to send alert %v to %v %v\n", ak, e.To, err)
		return
	}
	collect.Add("email.sent", nil, 1)
	slog.Infof("relayed alert %v to %v sucessfully. Subject: %d bytes. Body: %d bytes.", ak, e.To, len(e.Subject), len(e.HTML))
}

// Send an email using the given host and SMTP auth (optional), returns any
// error thrown by smtp.SendMail. This function merges the To, Cc, and Bcc
// fields and calls the smtp.SendMail function using the Email.Bytes() output as
// the message.
func Send(e *email.Email, addr, username, password string) error {
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
	return SendMail(addr, username, password, from.Address, to, raw)
}

// SendMail connects to the server at addr, switches to TLS if
// possible, authenticates with the optional mechanism a if possible,
// and then sends an email from address from, to addresses to, with
// message msg.
func SendMail(addr, username, password string, from string, to []string, msg []byte) error {
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
