package expr

import (
	"fmt"
	"reflect"
	"runtime"

	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/tsaf/expr/parse"
)

type Expr struct {
	*parse.Tree
}

func New(name, expr string) (*Expr, error) {
	t, err := parse.Parse(name, expr, Builtins)
	if err != nil {
		return nil, err
	}
	e := &Expr{
		Tree: t,
	}
	return e, nil
}

// Execute applies a parse expression to the specified OpenTSDB host, and
// returns one result per group.
func (e *Expr) Execute(host string) (r []*Result, err error) {
	defer errRecover(&err)
	r = e.walk(e.Tree.Root)
	return
}

// errRecover is the handler that turns panics into returns from the top
// level of Parse.
func errRecover(errp *error) {
	e := recover()
	if e != nil {
		switch err := e.(type) {
		case runtime.Error:
			panic(e)
		case error:
			*errp = err
		default:
			panic(e)
		}
	}
}

type Value float64

type Result struct {
	Value
	Group opentsdb.TagSet
}

type Union struct {
	A, B  Value
	Group opentsdb.TagSet
}

// wrap creates a new Result with a nil group and given value.
func wrap(v float64) []*Result {
	return []*Result{
		{
			Value: Value(v),
			Group: nil,
		},
	}
}

// union returns the combination of a and b sharing the same group
func union(a, b []*Result) []Union {
	var u []Union
	for _, ra := range a {
		for _, rb := range b {
			if ra.Group.Equal(rb.Group) || len(ra.Group) == 0 || len(rb.Group) == 0 {
				u = append(u, Union{
					A:     ra.Value,
					B:     rb.Value,
					Group: ra.Group,
				})
			}
		}
	}
	return u
}

func (e *Expr) walk(node parse.Node) []*Result {
	switch node := node.(type) {
	case *parse.BoolNode:
		return e.walk(node.Expr)
	case *parse.NumberNode:
		return wrap(node.Float64)
	case *parse.BinaryNode:
		return e.walkBinary(node)
	case *parse.UnaryNode:
		return e.walkUnary(node)
	case *parse.FuncNode:
		return e.walkFunc(node)
	default:
		panic(fmt.Errorf("expr: unknown node type"))
	}
}

func (e *Expr) walkBinary(node *parse.BinaryNode) []*Result {
	a := e.walk(node.Args[0])
	b := e.walk(node.Args[1])
	var res []*Result
	u := union(a, b)
	for _, v := range u {
		a := v.A
		b := v.B
		var r Value
		switch node.OpStr {
		case "+":
			r = a + b
		case "*":
			r = a * b
		case "-":
			r = a - b
		case "/":
			r = a / b
		case "==":
			if a == b {
				r = 1
			} else {
				r = 0
			}
		case ">":
			if a > b {
				r = 1
			} else {
				r = 0
			}
		case "!=":
			if a != b {
				r = 1
			} else {
				r = 0
			}
		case "<":
			if a < b {
				r = 1
			} else {
				r = 0
			}
		case ">=":
			if a >= b {
				r = 1
			} else {
				r = 0
			}
		case "<=":
			if a <= b {
				r = 1
			} else {
				r = 0
			}
		case "||":
			if a != 0 || b != 0 {
				r = 1
			} else {
				r = 0
			}
		case "&&":
			if a != 0 && b != 0 {
				r = 1
			} else {
				r = 0
			}
		default:
			panic(fmt.Errorf("expr: unknown operator %s", node.OpStr))
		}
		res = append(res, &Result{
			Value: r,
			Group: v.Group,
		})
	}
	return res
}

func (e *Expr) walkUnary(node *parse.UnaryNode) []*Result {
	a := e.walk(node.Arg)
	for _, r := range a {
		switch node.OpStr {
		case "!":
			if r.Value == 0 {
				r.Value = 1
			} else {
				r.Value = 0
			}
		case "-":
			r.Value = -r.Value
		default:
			panic(fmt.Errorf("expr: unknown operator %s", node.OpStr))
		}
	}
	return a
}

func (e *Expr) walkFunc(node *parse.FuncNode) []*Result {
	f := reflect.ValueOf(node.F.F)
	var in []reflect.Value
	for _, a := range node.Args {
		var v interface{}
		switch t := a.(type) {
		case *parse.StringNode:
			v = t.Text
		case *parse.NumberNode:
			v = t.Float64
		case *parse.QueryNode:
			v = t.Text
		default:
			panic(fmt.Errorf("expr: unknown func arg type"))
		}
		in = append(in, reflect.ValueOf(v))
	}
	ld := len(node.F.Args) - len(node.F.Defaults)
	for i, l := len(in), len(node.F.Args); i < l; i++ {
		d := node.F.Defaults[i-ld]
		in = append(in, reflect.ValueOf(d))
	}
	fr := f.Call(in)
	res := fr[0].Interface().([]*Result)
	if len(fr) > 1 && !fr[1].IsNil() {
		err := fr[1].Interface().(error)
		if err != nil {
			panic(err)
		}
	}
	return res
}
