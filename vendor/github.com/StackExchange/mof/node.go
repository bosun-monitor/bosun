// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Parse nodes.

package mof

import (
	"fmt"
	"strconv"
)

// A Node is an element in the parse tree. The interface is trivial.
// The interface contains an unexported method so that only
// types local to this package can satisfy it.
type node interface {
	Type() nodeType
	Value() interface{}
	Position() pos // byte position of start of node in full original input string
	// Make sure only functions in this package can create Nodes.
	unexported()
}

// NodeType identifies the type of a parse tree node.
type nodeType int

// Pos represents a byte position in the original input text from which
// this template was parsed.
type pos int

func (p pos) Position() pos {
	return p
}

// unexported keeps Node implementations local to the package.
// All implementations embed Pos, so this takes care of it.
func (pos) unexported() {
}

// Type returns itself and provides an easy default implementation
// for embedding in a Node. Embedded in all non-trivial Nodes.
func (t nodeType) Type() nodeType {
	return t
}

const (
	nodeClass nodeType = iota
	nodeInstance
	nodeArray
	nodeString
	nodeNumber
	nodeBool
	nodeNil
)

// Nodes.

// NumberNode holds a number: signed or unsigned integer or float.
// The value is parsed and stored under all the types that can represent the value.
// This simulates in a small amount of code the behavior of Go's ideal constants.
type numberNode struct {
	nodeType
	pos
	IsInt   bool    // Number has an unsigned integral value.
	IsFloat bool    // Number has a floating-point value.
	Int64   int64   // The unsigned integer value.
	Float64 float64 // The floating-point value.
	Text    string  // The original textual representation from the input.
}

func newNumber(pos pos, text string) (*numberNode, error) {
	n := &numberNode{nodeType: nodeNumber, pos: pos, Text: text}
	u, err := strconv.ParseInt(text, 0, 64) // will fail for -0.
	if err == nil {
		n.IsInt = true
		n.Int64 = u
	}
	// If an integer extraction succeeded, promote the float.
	if !n.IsInt {
		f, err := strconv.ParseFloat(text, 64)
		if err == nil {
			n.IsFloat = true
			n.Float64 = f
		}
	}
	if !n.IsInt && !n.IsFloat {
		return nil, fmt.Errorf("illegal number syntax: %q", text)
	}
	return n, nil
}

func (n *numberNode) Value() interface{} {
	if n.IsInt {
		return n.Int64
	}
	return n.Float64
}

// StringNode holds a string constant. The value has been "unquoted".
type stringNode struct {
	nodeType
	pos
	Quoted string // The original text of the string, with quotes.
	Text   string // The string, after quote processing.
}

func newString(pos pos, orig, text string) *stringNode {
	return &stringNode{nodeType: nodeString, pos: pos, Quoted: orig, Text: text}
}

func (s *stringNode) Value() interface{} {
	return s.Text
}

type boolNode struct {
	nodeType
	pos
	Val bool
}

func newBool(pos pos, val bool) *boolNode {
	return &boolNode{nodeType: nodeBool, pos: pos, Val: val}
}

func (b *boolNode) Value() interface{} {
	return b.Val
}

type nilNode struct {
	nodeType
	pos
}

func newNil(pos pos) *nilNode {
	return &nilNode{nodeType: nodeNil, pos: pos}
}

func (b *nilNode) Value() interface{} {
	return nil
}

type classNode struct {
	nodeType
	pos
	Members members
}

func newClass(pos pos) *classNode {
	return &classNode{
		nodeType: nodeClass,
		pos:      pos,
		Members:  make(members),
	}
}

type members map[string]node

func (c *classNode) Value() interface{} {
	return c.Members.Value()
}

func (m members) Value() interface{} {
	r := make(map[string]interface{}, len(m))
	for k, v := range m {
		r[k] = v.Value()
	}
	return r
}

type instanceNode struct {
	nodeType
	pos
	Members members
}

func newInstance(pos pos) *instanceNode {
	return &instanceNode{
		nodeType: nodeInstance,
		pos:      pos,
		Members:  make(members),
	}
}

func (i *instanceNode) Value() interface{} {
	return i.Members.Value()
}

type arrayNode struct {
	nodeType
	pos
	Array []node
}

func newArray(pos pos) *arrayNode {
	return &arrayNode{nodeType: nodeArray, pos: pos}
}

func (a *arrayNode) Value() interface{} {
	r := make([]interface{}, len(a.Array))
	for i, v := range a.Array {
		r[i] = v.Value()
	}
	return r
}

// Walk invokes f on n and sub-nodes of n.
func walk(n node, f func(node)) {
	f(n)
	switch n := n.(type) {
	case *arrayNode:
		for _, a := range n.Array {
			walk(a, f)
		}
	case *classNode:
		for _, a := range n.Members {
			walk(a, f)
		}
	case *numberNode, *stringNode:
		// Ignore.
	default:
		panic(fmt.Errorf("other type: %T", n))
	}
}
