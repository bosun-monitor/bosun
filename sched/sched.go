package sched

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/expr"
)

type Alert struct {
	Template
	Name       string
	Owner      string
	Crit, Warn *expr.Expr

	templateName string
	overrideName string
}

type Template struct {
	Name    string
	Body    string
	Subject string
}

type Schedule struct {
	Templates map[string]*Template
	Alerts    map[string]*Alert
}

func Run(c *conf.Conf) error {
	s, err := Load(c)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	fmt.Println(string(b), err)
	select {}
}

func Load(c *conf.Conf) (*Schedule, error) {
	s := Schedule{
		Templates: make(map[string]*Template),
		Alerts:    make(map[string]*Alert),
	}
	for k, v := range c.Sections {
		sp := strings.SplitN(k, ".", 2)
		if len(sp) != 2 {
			return nil, fmt.Errorf("sched: bad section type: %s", k)
		}
		switch sp[0] {
		case "template":
			if _, ok := s.Templates[sp[1]]; ok {
				return nil, fmt.Errorf("sched: duplicate template: %s", sp[1])
			}
			t, err := NewTemplate(sp[1], v["body"], v["subject"])
			if err != nil {
				return nil, err
			}
			s.Templates[sp[1]] = t
		case "alert":
			if _, ok := s.Alerts[sp[1]]; ok {
				return nil, fmt.Errorf("sched: duplicate alert: %s", sp[1])
			}
			var crit, warn *expr.Expr
			if cs, ok := v["crit"]; ok {
				c, err := expr.New(c.Name, cs)
				if err != nil {
					return nil, fmt.Errorf("%s: %s", err, cs)
				}
				crit = c
			}
			if cs, ok := v["warn"]; ok {
				c, err := expr.New(c.Name, cs)
				if err != nil {
					return nil, fmt.Errorf("%s: %s", err, cs)
				}
				warn = c
			}
			a, err := NewAlert(sp[1], v["owner"], v["template"], v["override"], crit, warn)
			if err != nil {
				return nil, err
			}
			s.Alerts[sp[0]] = a
		default:
			return nil, fmt.Errorf("sched: unknown section type: %s", sp[0])
		}
	}
	return &s, nil
}

func NewTemplate(name, body, subject string) (*Template, error) {
	t := Template{
		Name:    name,
		Body:    body,
		Subject: subject,
	}
	if t.Name == "" {
		return nil, fmt.Errorf("sched: missing template name")
	}
	if t.Body == "" {
		return nil, fmt.Errorf("sched: missing template body in template.%s", name)
	}
	return &t, nil
}

func NewAlert(name, owner, template, override string, crit, warn *expr.Expr) (*Alert, error) {
	a := Alert{
		Name:         name,
		Owner:        owner,
		Warn:         warn,
		Crit:         crit,
		templateName: template,
		overrideName: override,
	}
	if a.Name == "" {
		return nil, fmt.Errorf("sched: missing alert name")
	}
	if a.Crit == nil && a.Warn == nil {
		return nil, fmt.Errorf("sched: alert.%s missing crit or warn", name)
	}
	return &a, nil
}
