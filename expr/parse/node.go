// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Parse nodes.

package parse

import (
	"fmt"
	"strconv"
)

var textFormat = "%s" // Changed to "%q" in tests for better error messages.

// A Node is an element in the parse tree. The interface is trivial.
// The interface contains an unexported method so that only
// types local to this package can satisfy it.
type Node interface {
	Type() NodeType
	String() string
	// Copy does a deep copy of the Node and all its components.
	// To avoid type assertions, some XxxNodes also have specialized
	// CopyXxx methods that return *XxxNode.
	//Copy() Node
	Position() Pos // byte position of start of node in full original input string
	// Make sure only functions in this package can create Nodes.
	Check() error // performs type checking for itself and sub-nodes
	Return() funcType
	unexported()
}

// NodeType identifies the type of a parse tree node.
type NodeType int

// Pos represents a byte position in the original input text from which
// this template was parsed.
type Pos int

func (p Pos) Position() Pos {
	return p
}

// unexported keeps Node implementations local to the package.
// All implementations embed Pos, so this takes care of it.
func (Pos) unexported() {
}

// Type returns itself and provides an easy default implementation
// for embedding in a Node. Embedded in all non-trivial Nodes.
func (t NodeType) Type() NodeType {
	return t
}

const (
	NodeBool   NodeType = iota // Boolean expression.
	NodeFunc                   // A function call.
	NodeBinary                 // Binary operator: math, logical, compare
	NodeUnary                  // Unary operator: !, -
	NodeQuery                  // An OpenTSDB Query.
	NodeString                 // A string constant.
	NodeNumber                 // A numerical constant.
)

// Nodes.

// BoolNode holds a boolean expression node.
type BoolNode struct {
	NodeType
	Pos
	Expr Node // A NodeNumber or other Node yielding a scalar.
}

func newBool(pos Pos) *BoolNode {
	return &BoolNode{NodeType: NodeBool, Pos: pos}
}

func (l *BoolNode) String() string {
	return l.Expr.String()
}

func (b *BoolNode) Check() error {
	return b.Expr.Check()
}

func (b *BoolNode) Return() funcType { return TYPE_NUMBER }

// FuncNode holds a function invocation.
type FuncNode struct {
	NodeType
	Pos
	Name string
	F    Func
	Args []Node
}

func newFunc(pos Pos, name string, f Func) *FuncNode {
	return &FuncNode{NodeType: NodeFunc, Pos: pos, Name: name, F: f}
}

func (c *FuncNode) append(arg Node) {
	c.Args = append(c.Args, arg)
}

func (c *FuncNode) String() string {
	s := c.Name + "("
	for i, arg := range c.Args {
		if i > 0 {
			s += ", "
		}
		s += arg.String()
	}
	s += ")"
	return s
}

func (c *FuncNode) Check() error {
	const errFuncType = "parse: bad argument type in %s, expected %s, got %s"
	if len(c.Args) < len(c.F.Args)-c.F.Optional {
		return fmt.Errorf("parse: not enough arguments for %s", c.Name)
	} else if len(c.Args) > len(c.F.Args) {
		return fmt.Errorf("parse: too many arguments for %s", c.Name)
	}
	for i, a := range c.Args {
		t := c.F.Args[i]
		switch a.(type) {
		case *NumberNode:
			if t != TYPE_NUMBER {
				return fmt.Errorf(errFuncType, c.Name, t, "number")
			}
		case *StringNode:
			if t != TYPE_STRING {
				return fmt.Errorf(errFuncType, c.Name, t, "string")
			}
		case *QueryNode:
			if t != TYPE_QUERY && t != TYPE_SERIES {
				return fmt.Errorf(errFuncType, c.Name, t, "query")
			}
		}
	}
	return nil
}

func (f *FuncNode) Return() funcType { return f.F.Return }

// NumberNode holds a number: signed or unsigned integer or float.
// The value is parsed and stored under all the types that can represent the value.
// This simulates in a small amount of code the behavior of Go's ideal constants.
type NumberNode struct {
	NodeType
	Pos
	IsUint  bool    // Number has an unsigned integral value.
	IsFloat bool    // Number has a floating-point value.
	Uint64  uint64  // The unsigned integer value.
	Float64 float64 // The floating-point value.
	Text    string  // The original textual representation from the input.
}

func newNumber(pos Pos, text string) (*NumberNode, error) {
	n := &NumberNode{NodeType: NodeNumber, Pos: pos, Text: text}
	// Do integer test first so we get 0x123 etc.
	u, err := strconv.ParseUint(text, 0, 64) // will fail for -0.
	if err == nil {
		n.IsUint = true
		n.Uint64 = u
	}
	// If an integer extraction succeeded, promote the float.
	if n.IsUint {
		n.IsFloat = true
		n.Float64 = float64(n.Uint64)
	} else {
		f, err := strconv.ParseFloat(text, 64)
		if err == nil {
			n.IsFloat = true
			n.Float64 = f
			// If a floating-point extraction succeeded, extract the int if needed.
			if !n.IsUint && float64(uint64(f)) == f {
				n.IsUint = true
				n.Uint64 = uint64(f)
			}
		}
	}
	if !n.IsUint && !n.IsFloat {
		return nil, fmt.Errorf("illegal number syntax: %q", text)
	}
	return n, nil
}

func (n *NumberNode) String() string {
	return n.Text
}

func (n *NumberNode) Check() error {
	return nil
}

func (n *NumberNode) Return() funcType { return TYPE_NUMBER }

// StringNode holds a string constant. The value has been "unquoted".
type StringNode struct {
	NodeType
	Pos
	Quoted string // The original text of the string, with quotes.
	Text   string // The string, after quote processing.
}

func newString(pos Pos, orig, text string) *StringNode {
	return &StringNode{NodeType: NodeString, Pos: pos, Quoted: orig, Text: text}
}

func (s *StringNode) String() string {
	return s.Quoted
}

func (s *StringNode) Check() error {
	return nil
}

func (s *StringNode) Return() funcType { return TYPE_STRING }

// QueryNode holds a string constant. The value has been "unbracketed".
type QueryNode struct {
	NodeType
	Pos
	Bracketed string // The original text query, with brackets.
	Text      string // The string, after bracket processing.
}

func newQuery(pos Pos, orig, text string) *QueryNode {
	return &QueryNode{NodeType: NodeString, Pos: pos, Bracketed: orig, Text: text}
}

func (s *QueryNode) String() string {
	return s.Bracketed
}

func (q *QueryNode) Check() error {
	return nil
}

func (q *QueryNode) Return() funcType { return TYPE_QUERY }

// BinaryNode holds two arguments and an operator.
type BinaryNode struct {
	NodeType
	Pos
	Args     [2]Node
	Operator item
}

func newBinary(operator item, arg1, arg2 Node) *BinaryNode {
	return &BinaryNode{NodeType: NodeBinary, Pos: operator.pos, Args: [2]Node{arg1, arg2}, Operator: operator}
}

func (b *BinaryNode) String() string {
	return fmt.Sprintf("%s %s %s", b.Args[0], b.Operator.val, b.Args[1])
}

func (b *BinaryNode) Check() error {
	t1 := b.Args[0].Return()
	t2 := b.Args[1].Return()
	/* valid:
	n, n
	n, s
	*/
	if t2 == TYPE_NUMBER {
		t1, t2 = t2, t1
	}
	if t1 != TYPE_NUMBER {
		return fmt.Errorf("parse: type error in %s: at least one side must be a number", b)
	}
	if t2 != TYPE_NUMBER && !t2.IsSeries() {
		return fmt.Errorf("parse: type error in %s", b)
	}
	switch b.Operator.typ {
	case itemPlus, itemMinus, itemMult, itemDiv:
		// ignore
	default:
		if t2 != TYPE_NUMBER {
			return fmt.Errorf("parse: type error in %s: both sides must be numbers", b)
		}
	}

	if err := b.Args[0].Check(); err != nil {
		return err
	}
	return b.Args[1].Check()
}

func (b *BinaryNode) Return() funcType { return TYPE_NUMBER }

// UnaryNode holds one argument and an operator.
type UnaryNode struct {
	NodeType
	Pos
	Arg      Node
	Operator item
}

func newUnary(operator item, arg Node) *UnaryNode {
	return &UnaryNode{NodeType: NodeUnary, Pos: operator.pos, Arg: arg, Operator: operator}
}

func (u *UnaryNode) String() string {
	return fmt.Sprintf("%s%s", u.Operator.val, u.Arg)
}

func (u *UnaryNode) Check() error {
	if t := u.Arg.Return(); t != TYPE_NUMBER {
		return fmt.Errorf("parse: type error in %s, expected %s, got %s", u, "number", t)
	}
	return u.Arg.Check()
}

func (u *UnaryNode) Return() funcType { return TYPE_NUMBER }
