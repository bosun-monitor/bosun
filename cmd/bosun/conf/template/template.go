// Package template is a thin wrapper over go's text/template and html/template packages. It allows you to use either of them with a single api. Text or HTML are inferred from the template name
package template

import (
	"bytes"
	htemplate "html/template"
	"io"
	"strings"
	ttemplate "text/template"

	"github.com/aymerick/douceur/inliner"
)

type iface interface {
	Execute(io.Writer, interface{}) error
}

var _ htemplate.Template
var _ ttemplate.Template

type Template struct {
	inner  iface
	isHTML bool
}

type FuncMap map[string]interface{}

func New(name string) *Template {
	t := &Template{
		isHTML: isHTMLTemplate(name),
	}
	if t.isHTML {
		t.inner = htemplate.New(name)
	} else {
		t.inner = ttemplate.New(name)
	}
	return t
}

func (t *Template) Execute(w io.Writer, ctx interface{}) error {
	if t.isHTML {
		// inline css for html templates
		buf := &bytes.Buffer{}
		err := t.inner.Execute(buf, ctx)
		if err != nil {
			return err
		}
		s, err := inliner.Inline(buf.String())
		if err != nil {
			return err
		}
		if _, err = w.Write([]byte(s)); err != nil {
			return err
		}
	} else {
		return t.inner.Execute(w, ctx)
	}
	return nil
}

func (t *Template) copy(tmpl iface) *Template {
	return &Template{
		isHTML: t.isHTML,
		inner:  tmpl,
	}
}

func (t *Template) t() *ttemplate.Template {
	return t.inner.(*(ttemplate.Template))
}
func (t *Template) h() *htemplate.Template {
	return t.inner.(*htemplate.Template)
}

func Must(t *Template, err error) *Template {
	if err != nil {
		panic(err)
	}
	return t
}

func (t *Template) New(name string) *Template {
	if t.isHTML {
		return t.copy(t.h().New(name))
	}
	return t.copy(t.t().New(name))
}

func (t *Template) Funcs(fm FuncMap) *Template {
	if t.isHTML {
		return t.copy(t.h().Funcs(htemplate.FuncMap(fm)))
	}
	return t.copy(t.t().Funcs(ttemplate.FuncMap(fm)))
}

func (t *Template) Parse(text string) (*Template, error) {
	var i iface
	var err error
	if t.isHTML {
		i, err = t.h().Parse(text)
	} else {
		i, err = t.t().Parse(text)
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
