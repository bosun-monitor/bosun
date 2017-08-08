// Package template is a thin wrapper over go's text/template and html/template packages. It allows you to use either of them with a single api. Text or HTML are inferred from the template name
package template

import (
	htemplate "html/template"
	"io"
	"strings"
	ttemplate "text/template"
)

type iface interface {
	Execute(io.Writer, interface{}) error
}

var _ htemplate.Template
var _ ttemplate.Template

type Template struct {
	iface
	isHTML bool
}

type FuncMap map[string]interface{}

func New(name string) *Template {
	t := &Template{
		isHTML: isHTMLTemplate(name),
	}
	if t.isHTML {
		t.iface = htemplate.New(name)
	} else {
		t.iface = ttemplate.New(name)
	}
	return t
}

func (t *Template) copy(tmpl iface) *Template {
	return &Template{
		isHTML: t.isHTML,
		iface:  tmpl,
	}
}

func (t *Template) t() *ttemplate.Template {
	return t.iface.(*(ttemplate.Template))
}
func (t *Template) h() *htemplate.Template {
	return t.iface.(*htemplate.Template)
}

func (t *Template) New(name string) *Template {
	if t.isHTML {
		return t.copy(t.t().New(name))
	}
	return t.copy(t.h().New(name))
}

func (t *Template) Funcs(fm FuncMap) *Template {
	if t.isHTML {
		return t.copy(t.t().Funcs(ttemplate.FuncMap(fm)))
	}
	return t.copy(t.h().Funcs(htemplate.FuncMap(fm)))
}

func (t *Template) Parse(text string) (*Template, error) {
	var i iface
	var err error
	if t.isHTML {
		i, err = t.t().Parse(text)
	} else {
		i, err = t.h().Parse(text)
	}
	return t.copy(i), err
}

func isHTMLTemplate(name string) bool {
	name = strings.ToLower(name)
	if name == "emailbody" || name == "body" || strings.HasSuffix(name, "html") {
		return true
	}
	return false
}
