package expr

import (
	"fmt"
	"runtime"

	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/tsaf/expr/parse"
)

type Expr struct {
	*parse.Tree
}

func New(expr string) (*Expr, error) {
	t, err := parse.Parse(expr, expr, parse.Builtins)
	if err != nil {
		return nil, err
	}
	e := &Expr{
		Tree: t,
	}
	return e, nil
}

type Result struct {
	Result bool
	Group  opentsdb.TagSet
}

// Execute applies a parse expression to the specified OpenTSDB host, and
// returns one result per group.
func (e *Expr) Execute(host string) (r float64, err error) {
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

func (e *Expr) walk(node parse.Node) float64 {
	switch node := node.(type) {
	case *parse.BoolNode:
		return e.walk(node.Expr)
	case *parse.NumberNode:
		return node.Float64
	case *parse.BinaryNode:
		return e.walkBinary(node)
	case *parse.UnaryNode:
		return e.walkUnary(node)
	default:
		panic(fmt.Errorf("expr: unknown node type"))
	}
}

func (e *Expr) walkBinary(node *parse.BinaryNode) float64 {
	a := e.walk(node.Args[0])
	b := e.walk(node.Args[1])
	switch node.OpStr {
	case "+":
		return a + b
	case "*":
		return a * b
	case "-":
		return a - b
	case "/":
		return a / b
	case "==":
		if a == b {
			return 1
		}
		return 0
	case ">":
		if a > b {
			return 1
		}
		return 0
	case "!=":
		if a != b {
			return 1
		}
		return 0
	case "<":
		if a < b {
			return 1
		}
		return 0
	case ">=":
		if a >= b {
			return 1
		}
		return 0
	case "<=":
		if a <= b {
			return 1
		}
		return 0
	case "||":
		if a != 0 || b != 0 {
			return 1
		}
		return 0
	case "&&":
		if a != 0 && b != 0 {
			return 1
		}
		return 0
	default:
		panic(fmt.Errorf("expr: unknown operator %s", node.OpStr))
	}
}

func (e *Expr) walkUnary(node *parse.UnaryNode) float64 {
	a := e.walk(node.Arg)
	switch node.OpStr {
	case "!":
		if a == 0 {
			return 1
		}
		return 0
	case "-":
		return -a
	default:
		panic(fmt.Errorf("expr: unknown operator %s", node.OpStr))
	}
}
