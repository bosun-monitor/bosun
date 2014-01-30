package web

import (
	"fmt"
	"html/template"
	"strings"
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

func tagv(t string) string {
	return strings.Split(t, ".ds.stackexchange.com")[0]
}

var funcs = template.FuncMap{
	"status": status,
	"tagv":   tagv,
	"tsince": tsince,
}
