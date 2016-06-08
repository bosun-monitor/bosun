// Package boolq lets you build generic query expressions.

package boolq

import (
	"fmt"

	"github.com/kylebrandt/boolq/parse"
)

// An Asker is something that can be queried using boolq. The string passed
// to Ask will be the component in an expression. For example with the expression
// `(foo:bar AND baz:biz)` Ask will be called twice, once with the argument "foo:bar"
// and another time with the argument "baz:biz"
type Asker interface {
	Ask(string) (bool, error)
}

// AskExpr takes an expression and an Asker. It then parses the expression
// calling the Asker's Ask on expressions AskNodes and returns if the
// expression is true or not for the given asker.
func AskExpr(expr string, asker Asker) (bool, error) {
	q, err := parse.Parse(expr)
	if err != nil {
		return false, err
	}
	return walk(q.Root, asker)
}

// AskParsedExpr is like AskExpr but takes an expression that has already
// been parsed by parse.Parse on the expression. This is useful if you are calling
// the same expression multiple times.
func AskParsedExpr(q *Tree, asker Asker) (bool, error) {
	if q.Tree.Root == nil {
		return true, nil
	}
	return walk(q.Root, asker)
}

type Tree struct {
	*parse.Tree
}

// Parse parses an expression and returns the parsed expression.
// It can be used wtih AskParsedExpr
func Parse(text string) (*Tree, error) {
	tree := &Tree{}
	if text == "" {
		tree.Tree = &parse.Tree{}
		return tree, nil
	}
	var err error
	tree.Tree, err = parse.Parse(text)
	return tree, err
}

func walk(node parse.Node, asker Asker) (bool, error) {
	switch node := node.(type) {
	case *parse.AskNode:
		return asker.Ask(node.Text)
	case *parse.BinaryNode:
		return walkBinary(node, asker)
	case *parse.UnaryNode:
		return walkUnary(node, asker)
	default:
		return false, fmt.Errorf("can not walk type %v", node.Type())
	}
}

func walkBinary(node *parse.BinaryNode, asker Asker) (bool, error) {
	l, err := walk(node.Args[0], asker)
	if err != nil {
		return false, err
	}
	r, err := walk(node.Args[1], asker)
	if err != nil {
		return false, err
	}
	if node.OpStr == "AND" {
		return l && r, nil
	}
	if node.OpStr == "OR" {
		return l || r, nil
	}
	return false, fmt.Errorf("Unrecognized operator: %v", node.OpStr)
}

func walkUnary(node *parse.UnaryNode, asker Asker) (bool, error) {
	r, err := walk(node.Arg, asker)
	if err != nil {
		return false, err
	}
	if node.OpStr == "!" {
		return !r, nil
	}
	return false, fmt.Errorf("unknown unary operator: %v", node.OpStr)
}
