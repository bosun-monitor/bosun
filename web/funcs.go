package web

import (
	"html/template"

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

var funcs = template.FuncMap{
	"status": status,
}
