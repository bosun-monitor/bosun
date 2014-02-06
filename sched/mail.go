package sched

import (
	"bytes"
	"crypto/tls"
	"errors"
	"log"
	"net/mail"
	"net/smtp"
	"strings"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/conf"
	"github.com/jordan-wright/email"
)

func (s *Schedule) Email(name string, group opentsdb.TagSet) {
	a := s.Conf.Alerts[name]
	if a == nil {
		log.Println("sched: unknown alert name during email:", name)
	}
	if a.Owner == "" {
		return
	}
	body := new(bytes.Buffer)
	subject := new(bytes.Buffer)
	data := struct {
		Alert *conf.Alert
		Tags  opentsdb.TagSet
	}{
		a,
		group,
	}
	if a.Template.Body != nil {
		err := a.Template.Body.Execute(body, &data)
		if err != nil {
			log.Println(err)
			return
		}
	}
	if a.Template.Subject != nil {
		err := a.Template.Subject.Execute(subject, &data)
		if err != nil {
			log.Println(err)
			return
		}
	}
	e := email.NewEmail()
	e.From = "tsaf@stackexchange.com"
	e.To = strings.Split(a.Owner, ",")
	e.Subject = subject.String()
	e.Text = body.Bytes()
	err := Send(e, s.SmtpHost)
	if err != nil {
		log.Println(err)
		return
	}
	ak := AlertKey{a.Name, group.String()}
	state := s.Status[ak]
	state.Emailed = true
}

// Send an email using the given host and SMTP auth (optional), returns any error thrown by smtp.SendMail
// This function merges the To, Cc, and Bcc fields and calls the smtp.SendMail function using the Email.Bytes() output as the message
func Send(e *email.Email, addr string) error {
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
	return SendMail(addr, from.Address, to, raw)
}

// SendMail connects to the server at addr, switches to TLS if
// possible, authenticates with the optional mechanism a if possible,
// and then sends an email from address from, to addresses to, with
// message msg.
func SendMail(addr string, from string, to []string, msg []byte) error {
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
