package web

import (
	"fmt"
	"html/template"
	"time"
	"github.com/StackExchange/tsaf/sched"
)

func status(s sched.Status) string {
	switch s {
	case sched.ST_WARN:
		return "warning"
	case sched.ST_CRIT:
		return "danger"
	}
	return ""
}

func tsince(t time.Time) template.HTML {
	const tfmt = "1/2/2006 15:04:05 MST"
	s := time.Since(t)
	s = (s / time.Second) * time.Second
	return template.HTML(fmt.Sprintf("%s<br>%s ago", t.Format(tfmt), s))
}

var funcs = template.FuncMap{
	"status": status,
	"tsince": tsince,
}
