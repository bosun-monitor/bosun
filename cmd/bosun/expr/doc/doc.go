package doc

import (
	"fmt"
	"strings"
	"text/template"

	"bytes"

	"bosun.org/models"
)

// Func contains the documentation for an expression function
type Func struct {
	Name      string
	Summary   string
	Arguments Arguments
	Examples  []string
	Return    models.FuncType
}

type Funcs []*Func

type Docs map[string]Funcs

// Arguments is a slice of Arg objects
type Arguments []Arg

// Arg contains fields that document the arguments to a function
type Arg struct {
	Name string
	Desc string
	Type models.FuncType
}

// HasDescription returns true if any of the Arguments have a description
func (args Arguments) HasDescription() bool {
	for _, a := range args {
		if a.Desc != "" {
			return true
		}
	}
	return false
}

func (a Arg) Signature() string {
	return fmt.Sprintf("%v %v", a.Name, suffixSet(a.Type))
}

func (f Func) Signature() string {
	args := make([]string, len(f.Arguments))
	for i, arg := range f.Arguments {
		args[i] = arg.Signature()
	}
	return fmt.Sprintf("%v(%v) %v", f.Name, strings.Join(args, ","), suffixSet(f.Return))
}

func (a Arguments) TypeSlice() []models.FuncType {
	types := make([]models.FuncType, len(a))
	for i, arg := range a {
		types[i] = arg.Type
	}
	return types
}

func (fs Funcs) Signatures() []string {
	s := make([]string, len(fs))
	for i, f := range fs {
		s[i] = f.Signature()
	}
	return s
}

// TODO Just change the String method on models.FuncType
// Need to make sure all JS stuff (like grafanaplugin and this jscode) understands the change
func suffixSet(t models.FuncType) string {
	if strings.HasPrefix(t.String(), "series") || strings.HasPrefix(t.String(), "number") {
		return fmt.Sprintf("%vSet", t.String())
	}
	return t.String()
}

// TODO make series seriesSet, will also probably just makes these HTML
var funcWikiTemplate = `## {{ .Signature }}
{: .exprFunc}

{{ .Summary }}
{{ if .Arguments.HasDescription }}
Argument Details:
	{{ range $i, $arg := .Arguments -}}
		{{- if ne $arg.Desc "" }}
  * {{ $arg.Name }} ({{ $arg.Type }}): {{ $arg.Desc }}
		{{- end -}}
	{{- end -}}
{{- end -}}
`

var docWikiTemplate = `# Builtins, yay
{{ range $f := .builtins }}
{{ template "func" $f}}
{{ end }}

# Reduction Funcs, yay
{{ range $i, $f := .reduction }}
{{ template "func" $f}}
{{ end }}
`

func (d *Docs) WikiText() (bytes.Buffer, error) {
	var b bytes.Buffer
	t, err := template.New("func").Parse(funcWikiTemplate)
	if err != nil {
		return b, err
	}
	t, err = t.New("docs").Parse(docWikiTemplate)
	if err != nil {
		return b, err
	}
	err = t.Execute(&b, d)
	return b, nil
}