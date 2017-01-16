package conf

import (
	"bytes"
	"crypto/tls"
	"errors"
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

type NotificationMessage struct {
	Vars        map[string]interface{}
	Subject     string
	Body        string
	Payload     string
	Attachments []*models.Attachment

	Notification *Notification
}

// HACK This is quite a dirty way to maintain backward compatibility and support
// {{js .}} substitution of body/subject inside Notification body template for POST.
func (nm NotificationMessage) String() string {
	request := nm.Subject
	if nm.Notification.UseBody && nm.Body != "" {
		request = nm.Body
	} else if nm.Notification.UsePayload && nm.Payload != "" {
		request = nm.Payload
	}

	if request == "" {
		request = "Template subsection is empty"
		slog.Warningln("No content to send in notification message")
	}

	return request
}

func (n *Notification) Notify(message NotificationMessage, c SystemConfProvider, ak string) {
	m := message
	m.Notification = n
	if len(n.Email) > 0 {
		go n.DoEmail(m, c, ak)
	}
	if n.Post != nil {
		go n.DoPost(m, ak)
	}
	if n.Get != nil {
		go n.DoGet(m, ak)
	}
}

func (n *Notification) DoPrint(payload string) {
	slog.Infoln(payload)
}

func (n *Notification) DoPost(message NotificationMessage, ak string) {
	request := message.String()

	// Notification's body provides template for API integrations
	if n.Body != nil {
		buf := new(bytes.Buffer)
		err := n.Body.Execute(buf, message)
		if err != nil {
			slog.Errorln(err)
			return
		}
		request = buf.String()
	}

	resp, err := http.Post(n.Post.String(), n.ContentType, bytes.NewBuffer([]byte(request)))
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		slog.Error(err)
		return
	}
	if resp.StatusCode >= 300 {
		slog.Errorln("bad response on notification post:", resp.Status)
	} else {
		slog.Infof("post notification successful for alert %s. Response code %d.", ak, resp.StatusCode)
	}

	if n.Print {
		go n.DoPrint("POST payload: " + string(request))
	}
}

func (n *Notification) DoGet(message NotificationMessage, ak string) {
	resp, err := http.Get(n.Get.String())
	if err != nil {
		slog.Error(err)
		return
	}
	if resp.StatusCode >= 300 {
		slog.Error("bad response on notification get:", resp.Status)
	} else {
		slog.Infof("get notification successful for alert %s. Response code %d.", ak, resp.StatusCode)
	}
}

func (n *Notification) DoEmail(message NotificationMessage, c SystemConfProvider, ak string) {
	e := email.NewEmail()
	e.From = c.GetEmailFrom()
	for _, a := range n.Email {
		e.To = append(e.To, a.Address)
	}
	e.Subject = string(message.Subject)
	e.HTML = []byte(message.Body)

	// TODO: Check if we do extensive data copying here
	for _, a := range message.Attachments {
		e.Attach(bytes.NewBuffer(a.Data), a.Filename, a.ContentType)
	}
	e.Headers.Add("X-Bosun-Server", util.Hostname)
	if err := Send(e, c.GetSMTPHost(), c.GetSMTPUsername(), c.GetSMTPPassword()); err != nil {
		collect.Add("email.sent_failed", nil, 1)
		slog.Errorf("failed to send alert %v to %v %v\n", ak, e.To, err)
		return
	}
	collect.Add("email.sent", nil, 1)
	slog.Infof("relayed alert %v to %v sucessfully. Subject: %d bytes. Body: %d bytes.", ak, e.To,
		len(message.Subject), len(message.Body))

	if n.Print {
		go n.DoPrint("Subject: " + message.Subject + ", Body: " + message.Body)
	}
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
