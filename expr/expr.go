package expr

import (
	"fmt"
	"reflect"
	"runtime"

	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/expr/parse"
)

type state struct {
	*Expr
	host string
}

type Expr struct {
	*parse.Tree
}

func New(expr string) (*Expr, error) {
	t, err := parse.Parse(expr, Builtins)
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
func (e *Expr) Execute(host string, T miniprofiler.Timer) (r []*Result, err error) {
	defer errRecover(&err)
	s := &state{
		e,
		host,
	}
	if T == nil {
		T = new(miniprofiler.Profile)
	}
	T.Step("expr execute", func(T miniprofiler.Timer) {
		r = s.walk(e.Tree.Root, T)
	})
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

type Value interface {
	Type() parse.FuncType
	Value() interface{}
}

type Number float64

func (n Number) Type() parse.FuncType { return parse.TYPE_NUMBER }
func (n Number) Value() interface{}   { return n }

type Series map[string]opentsdb.Point

func (s Series) Type() parse.FuncType { return parse.TYPE_SERIES }
func (s Series) Value() interface{}   { return s }

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
			Value: Number(v),
			Group: nil,
		},
	}
}

// union returns the combination of a and b where one is a strict subset of the
// other.
func union(a, b []*Result) []Union {
	var u []Union
	for _, ra := range a {
		for _, rb := range b {
			if ra.Group.Equal(rb.Group) || len(ra.Group) == 0 || len(rb.Group) == 0 {
				g := ra.Group
				if len(ra.Group) == 0 {
					g = rb.Group
				}
				u = append(u, Union{
					A:     ra.Value,
					B:     rb.Value,
					Group: g,
				})
			} else if ra.Group.Subset(rb.Group) {
				u = append(u, Union{
					A:     ra.Value,
					B:     rb.Value,
					Group: rb.Group,
				})
			} else if rb.Group.Subset(ra.Group) {
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

func (e *state) walk(node parse.Node, T miniprofiler.Timer) []*Result {
	switch node := node.(type) {
	case *parse.NumberNode:
		return wrap(node.Float64)
	case *parse.BinaryNode:
		return e.walkBinary(node, T)
	case *parse.UnaryNode:
		return e.walkUnary(node, T)
	case *parse.FuncNode:
		return e.walkFunc(node, T)
	default:
		panic(fmt.Errorf("expr: unknown node type"))
	}
}

func (e *state) walkBinary(node *parse.BinaryNode, T miniprofiler.Timer) []*Result {
	ar := e.walk(node.Args[0], T)
	br := e.walk(node.Args[1], T)
	var res []*Result
	u := union(ar, br)
	for _, v := range u {
		var r Value
		switch at := v.A.(type) {
		case Number:
			switch bt := v.B.(type) {
			case Number:
				r = Number(operate(node.OpStr, float64(at), float64(bt)))
			default:
				panic("expr: unknown op type")
			}
		default:
			panic("expr: unknown op type")
		}
		res = append(res, &Result{
			Value: r,
			Group: v.Group,
		})
	}
	return res
}

func operate(op string, a, b float64) (r float64) {
	switch op {
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
		panic(fmt.Errorf("expr: unknown operator %s", op))
	}
	return
}

func (e *state) walkUnary(node *parse.UnaryNode, T miniprofiler.Timer) []*Result {
	a := e.walk(node.Arg, T)
	for _, r := range a {
		switch rt := r.Value.(type) {
		case Number:
			r.Value = Number(uoperate(node.OpStr, float64(rt)))
		default:
			panic(fmt.Errorf("expr: unknown op type"))
		}
	}
	return a
}

func uoperate(op string, a float64) (r float64) {
	switch op {
	case "!":
		if a == 0 {
			r = 1
		} else {
			r = 0
		}
	case "-":
		r = -a
	default:
		panic(fmt.Errorf("expr: unknown operator %s", op))
	}
	return
}

func (e *state) walkFunc(node *parse.FuncNode, T miniprofiler.Timer) []*Result {
	f := reflect.ValueOf(node.F.F)
	var in []reflect.Value
	for _, a := range node.Args {
		var v interface{}
		switch t := a.(type) {
		case *parse.StringNode:
			v = t.Text
		case *parse.NumberNode:
			v = t.Float64
		case *parse.FuncNode:
			v = e.walkFunc(t, T)
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
	fr := f.Call(append([]reflect.Value{reflect.ValueOf(e), reflect.ValueOf(T)}, in...))
	res := fr[0].Interface().([]*Result)
	if len(fr) > 1 && !fr[1].IsNil() {
		err := fr[1].Interface().(error)
		if err != nil {
			panic(err)
		}
	}
	return res
}
