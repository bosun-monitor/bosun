// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Parse nodes.
package parse

import "fmt"

var textFormat = "%s" // Changed to "%q" in tests for better error messages.

// A Node is an element in the parse tree. The interface is trivial.
// The interface contains an unexported method so that only
// types local to this package can satisfy it.
type Node interface {
	Type() NodeType
	String() string
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
	NodeAsk    NodeType = iota // key:value expression.
	NodeBinary NodeType = iota //
	NodeUnary  NodeType = iota //
)

// BinaryNode holds two arguments and an operator.
type BinaryNode struct {
	NodeType
	Pos
	Args     [2]Node
	Operator item
	OpStr    string
}

func newBinary(operator item, arg1, arg2 Node) *BinaryNode {
	return &BinaryNode{NodeType: NodeBinary, Pos: operator.pos, Args: [2]Node{arg1, arg2}, Operator: operator, OpStr: operator.val}
}

func (b *BinaryNode) String() string {
	return fmt.Sprintf("%s %s %s", b.Args[0], b.Operator.val, b.Args[1])
}

func (b *BinaryNode) StringAST() string {
	return fmt.Sprintf("%s(%s, %s)", b.Operator.val, b.Args[0], b.Args[1])
}

// UnaryNode holds one argument and an operator.
type UnaryNode struct {
	NodeType
	Pos
	Arg      Node
	Operator item
	OpStr    string
}

func newUnary(operator item, arg Node) *UnaryNode {
	return &UnaryNode{NodeType: NodeUnary, Pos: operator.pos, Arg: arg, Operator: operator, OpStr: operator.val}
}

func (u *UnaryNode) String() string {
	return fmt.Sprintf("%s%s", u.Operator.val, u.Arg)
}

func (u *UnaryNode) StringAST() string {
	return fmt.Sprintf("%s(%s)", u.Operator.val, u.Arg)
}

// Walk invokes f on n and sub-nodes of n.
func Walk(n Node, f func(Node)) {
	f(n)
	switch n := n.(type) {
	case *BinaryNode:
		Walk(n.Args[0], f)
		Walk(n.Args[1], f)
	case *AskNode:
		// Ignore.
	case *UnaryNode:
		Walk(n.Arg, f)
	default:
		panic(fmt.Errorf("other type: %T", n))
	}
}

// AskNode holds a filter invocation.
type AskNode struct {
	NodeType
	Pos
	Text string
}

func (a *AskNode) String() string {
	return fmt.Sprintf("%s", a.Text)
}

func newAsk(pos Pos, text string) *AskNode {
	return &AskNode{
		NodeType: NodeAsk,
		Pos:      pos,
		Text:     text,
	}
}