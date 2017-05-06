package doc

import (
	"fmt"
	"strings"

	"bosun.org/models"
)

// Func contains the documentation for an expression function
type Func struct {
	Name      string
	Summary   string
	Arguments Arguments
	Return    models.FuncType
}

// Arguments is a slice of Arg objects
type Arguments []Arg

// Arg contains fields that document the arguments to a function
type Arg struct {
	Name string
	Desc string
	Type models.FuncType
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

// TODO Just change the String method on models.FuncType
// Need to make sure all JS stuff (like grafanaplugin and this jscode) understands the change
func suffixSet(t models.FuncType) string {
	if strings.HasPrefix(t.String(), "series") || strings.HasPrefix(t.String(), "number") {
		return fmt.Sprintf("%vSet", t.String())
	}
	return t.String()
}
