// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Parse nodes.

package expr

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

// FuncNode holds a function invocation.
type FuncNode struct {
	NodeType
	Pos
	Name string
	Args []Node
}

func newFunc(pos Pos, name string) *FuncNode {
	return &FuncNode{NodeType: NodeFunc, Pos: pos, Name: name}
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
