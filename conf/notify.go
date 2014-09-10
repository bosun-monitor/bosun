package conf

import (
	"bytes"
	"crypto/tls"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"net/smtp"

	"github.com/StackExchange/bosun/_third_party/github.com/jordan-wright/email"
)

func (n *Notification) Notify(subject, body []byte, from, smtpHost string) {
	if len(n.Email) > 0 {
		go n.DoEmail(subject, body, from, smtpHost)
	}
	if n.Post != nil {
		go n.DoPost(subject)
	}
	if n.Get != nil {
		go n.DoGet()
	}
	if n.Print {
		go n.DoPrint(subject)
	}
}

func (n *Notification) DoPrint(subject []byte) {
	log.Println(string(subject))
}

func (n *Notification) DoPost(subject []byte) {
	if n.Body != nil {
		buf := new(bytes.Buffer)
		if err := n.Body.Execute(buf, subject); err != nil {
			log.Println(err)
			return
		}
		subject = buf.Bytes()
	}
	resp, err := http.Post(n.Post.String(), "application/x-www-form-urlencoded", bytes.NewBuffer(subject))
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		log.Println(err)
		return
	}
	if resp.StatusCode >= 300 {
		log.Println("bad response on notification post:", resp.Status)
	}
}

func (n *Notification) DoGet() {
	resp, err := http.Get(n.Get.String())
	if err != nil {
		log.Println(err)
		return
	}
	if resp.StatusCode >= 300 {
		log.Println("bad response on notification get:", resp.Status)
	}
}

func (n *Notification) DoEmail(subject, body []byte, from, smtpHost string) {
	e := email.NewEmail()
	e.From = from
	for _, a := range n.Email {
		e.To = append(e.To, a.Address)
	}
	e.Subject = string(subject)
	e.HTML = body
	if err := Send(e, smtpHost); err != nil {
		log.Println(err)
		return
	}
}

// Send an email using the given host and SMTP auth (optional), returns any
// error thrown by smtp.SendMail. This function merges the To, Cc, and Bcc
// fields and calls the smtp.SendMail function using the Email.Bytes() output as
// the message.
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
