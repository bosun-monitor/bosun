package sched

import (
	"bytes"
	"log"
	"strings"

	"github.com/StackExchange/tcollector/opentsdb"
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
	e.Text = body.String()
	err := e.Send(s.SmtpHost, nil)
	if err != nil {
		log.Println(err)
		return
	}
	ak := AlertKey{a.Name, group.String()}
	state := s.Status[ak]
	state.Emailed = true
}
