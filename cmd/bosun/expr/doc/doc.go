package doc

import (
	"fmt"
	"strings"

	"bosun.org/models"
)

// Func contains the documentation for an expression function
type Func struct {
	Name        string
	Description string
	Arguments   Arguments
	Return      models.FuncType
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
	return fmt.Sprintf("%v %v", a.Name, a.Type)
}

func (f Func) Signature() string {
	args := make([]string, len(f.Arguments))
	for i, arg := range f.Arguments {
		args[i] = arg.Signature()
	}
	return fmt.Sprintf("%v(%v) %v", f.Name, strings.Join(args, ","), f.Return)
}
