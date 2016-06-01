// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package parse builds parse trees for expressions as defined by expr. Clients
// should use that package to construct expressions rather than this one, which
// provides shared internal data structures not intended for general use.
package parse

import (
	"fmt"
	"runtime"
)

// Tree is the representation of a single parsed expression.
type Tree struct {
	Text string // text parsed to create the expression.
	Root Node   // top-level root of the tree, returns a number.

	// Parsing only; cleared after parse.
	lex       *lexer
	token     [1]item // one-token lookahead for parser.
	peekCount int
}

// Parse returns a Tree, created by parsing the expression described in the
// argument string. If an error is encountered, parsing stops and an empty Tree
// is returned with the error.
func Parse(text string) (t *Tree, err error) {
	t = New()
	t.Text = text
	err = t.Parse(text)
	return
}

// next returns the next token.
func (t *Tree) next() item {
	if t.peekCount > 0 {
		t.peekCount--
	} else {
		t.token[0] = t.lex.nextItem()
	}
	return t.token[t.peekCount]
}

// backup backs the input stream up one token.
func (t *Tree) backup() {
	t.peekCount++
}

// peek returns but does not consume the next token.
func (t *Tree) peek() item {
	if t.peekCount > 0 {
		return t.token[t.peekCount-1]
	}
	t.peekCount = 1
	t.token[0] = t.lex.nextItem()
	return t.token[0]
}

// Parsing.

// New allocates a new parse tree with the given name.
func New() *Tree {
	return &Tree{}
}

// errorf formats the error and terminates processing.
func (t *Tree) errorf(format string, args ...interface{}) {
	t.Root = nil
	format = fmt.Sprintf("expr: %s", format)
	panic(fmt.Errorf(format, args...))
}

// error terminates processing.
func (t *Tree) error(err error) {
	t.errorf("%s", err)
}

// expect consumes the next token and guarantees it has the required type.
func (t *Tree) expect(expected itemType, context string) item {
	token := t.next()
	if token.typ != expected {
		t.unexpected(token, context)
	}
	return token
}

// expectOneOf consumes the next token and guarantees it has one of the required types.
func (t *Tree) expectOneOf(expected1, expected2 itemType, context string) item {
	token := t.next()
	if token.typ != expected1 && token.typ != expected2 {
		t.unexpected(token, context)
	}
	return token
}

// unexpected complains about the token and terminates processing.
func (t *Tree) unexpected(token item, context string) {
	t.errorf("unexpected %s in %s", token, context)
}

// recover is the handler that turns panics into returns from the top level of Parse.
func (t *Tree) recover(errp *error) {
	e := recover()
	if e != nil {
		if _, ok := e.(runtime.Error); ok {
			panic(e)
		}
		if t != nil {
			t.stopParse()
		}
		*errp = e.(error)
	}
	return
}

// startParse initializes the parser, using the lexer.
func (t *Tree) startParse(lex *lexer) {
	t.Root = nil
	t.lex = lex
}

// stopParse terminates parsing.
func (t *Tree) stopParse() {
	t.lex = nil
}

// Parse parses the expression definition string to construct a representation
// of the expression for execution.
func (t *Tree) Parse(text string) (err error) {
	defer t.recover(&err)
	t.startParse(lex(text))
	t.Text = text
	t.parse()
	t.stopParse()
	return nil
}

// parse is the top-level parser for an expression.
// It runs to EOF.
func (t *Tree) parse() {
	t.Root = t.O()
	t.expect(itemEOF, "input")
}

/* Grammar:
O -> A {"AND" A}
A -> C {"OR" C}
C -> v | "(" O ")" | "!" O
v -> ask
*/

// expr:
func (t *Tree) O() Node {
	n := t.A()
	for {
		switch t.peek().typ {
		case itemOr:
			n = newBinary(t.next(), n, t.A())
		default:
			return n
		}
	}
}

func (t *Tree) A() Node {
	n := t.C()
	for {
		switch t.peek().typ {
		case itemAnd:
			n = newBinary(t.next(), n, t.C())
		default:
			return n
		}
	}
}

func (t *Tree) C() Node {
	switch token := t.peek(); token.typ {
	case itemAsk:
		return t.v()
	case itemNot:
		return newUnary(t.next(), t.C())
	case itemLeftParen:
		t.next()
		n := t.O()
		t.expect(itemRightParen, "input")
		return n
	default:
		t.unexpected(token, "input")
	}
	return nil
}

func (t *Tree) v() Node {
	switch token := t.next(); token.typ {
	case itemAsk:
		return newAsk(token.pos, token.val)
	default:
		t.unexpected(token, "input")
	}
	return nil
}
