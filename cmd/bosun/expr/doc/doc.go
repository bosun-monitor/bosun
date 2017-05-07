package doc

import (
	"fmt"
	"html/template"
	"reflect"
	"runtime"
	"strings"

	"bytes"

	"bosun.org/models"
)

type HTMLString string

func (h HTMLString) HTML() template.HTML {
	return template.HTML(h)
}

// Func contains the documentation for an expression function
type Func struct {
	Name      string
	Summary   HTMLString
	CodeLink  string
	Arguments Arguments
	Examples  []HTMLString
	Return    models.FuncType
}

type Funcs []*Func

type Docs map[string]Funcs

// Arguments is a slice of Arg objects
type Arguments []Arg

// Arg contains fields that document the arguments to a function
type Arg struct {
	Name string
	Desc HTMLString
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
	return fmt.Sprintf("%v(%v) %v", f.Name, strings.Join(args, ", "), suffixSet(f.Return))
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
var funcWikiTemplate = `
<p>{{ .Summary.HTML }}</p>
Code: {{ .CodeLink }}
{{ if .Arguments.HasDescription }}
Argument Details:
<ul>
	{{ range $i, $arg := .Arguments -}}
		{{- if ne $arg.Desc "" }}
	<li>{{ $arg.Name }} ({{ $arg.Type }}): {{ $arg.Desc.HTML }}</li>
		{{- end -}}
	{{- end }}
</ul>
{{- end -}}
`

var docWikiTemplate = `
<h1>Builtins</h1>
{{ range $f := .builtins }}
	<h2 class="exprFunc anchor">{{ $f.Signature }}</h2>
	{{ template "func" $f}}
{{ end }}

<h1>Reduction Functions</h1>
{{ range $i, $f := .reduction }}
	<h2 class="exprFunc anchor">{{ $f.Signature }}</h2>
	{{ template "func" $f}}
{{ end }}
`

func (d *Docs) Wiki() (bytes.Buffer, error) {
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

// SetCodeLink sets the Func's CodeLink to a url that points to the code in Github that corresponds to the passed function
func (f *Func) SetCodeLink(i interface{}) error {
	ptr := reflect.ValueOf(i).Pointer()
	file, no := runtime.FuncForPC(ptr).FileLine(ptr)
	idx := strings.LastIndex(file, "/cmd/") // Assuming non-nested cmd folders
	if idx < 0 {
		return fmt.Errorf("error setting code link, failed to trim path")
	}
	file = file[idx:]
	f.CodeLink = fmt.Sprintf("https://github.com/bosun-monitor/bosun/blob/master%v#L%v", file, no)
	return nil
}
