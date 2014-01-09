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
	"strconv"
)

// Tree is the representation of a single parsed expression.
type Tree struct {
	Text string    // text parsed to create the expression
	Root *BoolNode // top-level root of the tree.
	// Parsing only; cleared after parse.
	funcs     []map[string]Func
	lex       *lexer
	token     [1]item // one-token lookahead for parser.
	peekCount int
}

type Func struct {
	Args     []FuncType
	Return   FuncType
	Defaults []interface{}
	F        interface{}
}

func (f Func) Optional() int {
	return len(f.Defaults)
}

type FuncType int

func (f FuncType) String() string {
	switch f {
	case TYPE_NUMBER:
		return "number"
	case TYPE_STRING:
		return "string"
	case TYPE_QUERY:
		return "query"
	case TYPE_SERIES:
		return "series"
	default:
		return "unknown"
	}
}

func (f FuncType) IsSeries() bool {
	return f == TYPE_QUERY || f == TYPE_SERIES
}

const (
	TYPE_NUMBER FuncType = iota
	TYPE_STRING
	TYPE_QUERY
	TYPE_SERIES
)

// Parse returns a Tree, created by parsing the expression described in the
// argument string. If an error is encountered, parsing stops and an empty Tree
// is returned with the error.
func Parse(text string, funcs ...map[string]Func) (t *Tree, err error) {
	t = New()
	t.Text = text
	err = t.Parse(text, funcs...)
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
func New(funcs ...map[string]Func) *Tree {
	return &Tree{
		funcs: funcs,
	}
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
func (t *Tree) startParse(funcs []map[string]Func, lex *lexer) {
	t.Root = nil
	t.lex = lex
	t.funcs = funcs
}

// stopParse terminates parsing.
func (t *Tree) stopParse() {
	t.lex = nil
	t.funcs = nil
}

// Parse parses the template definition string to construct a representation of
// the template for execution. If either action delimiter string is empty, the
// default ("{{" or "}}") is used. Embedded template definitions are added to
// the treeSet map.
func (t *Tree) Parse(text string, funcs ...map[string]Func) (err error) {
	defer t.recover(&err)
	t.startParse(funcs, lex(text))
	t.Text = text
	t.parse()
	t.stopParse()
	return nil
}

// parse is the top-level parser for a template.
// It runs to EOF.
func (t *Tree) parse() {
	t.Root = newBool(t.peek().pos)
	t.Root.Expr = t.O()
	t.expect(itemEOF, "input")
	if err := t.Root.Check(); err != nil {
		t.error(err)
	}
}

/* Grammar:
O -> A {"||" A}
A -> C {"&&" C}
C -> P {( "==" | "!=" | ">" | ">=" | "<" | "<=") P}
P -> M {( "+" | "-" ) M}
M -> F {( "*" | "/" ) F}
F -> v | "(" O ")" | "!" O | "-" O
v -> number | func(..)
Func -> name "(" param {"," param} ")"
param -> number | "string" | [query]
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
	n := t.P()
	for {
		switch t.peek().typ {
		case itemEq, itemNotEq, itemGreater, itemGreaterEq, itemLess, itemLessEq:
			n = newBinary(t.next(), n, t.P())
		default:
			return n
		}
	}
}

func (t *Tree) P() Node {
	n := t.M()
	for {
		switch t.peek().typ {
		case itemPlus, itemMinus:
			n = newBinary(t.next(), n, t.M())
		default:
			return n
		}
	}
}

func (t *Tree) M() Node {
	n := t.F()
	for {
		switch t.peek().typ {
		case itemMult, itemDiv:
			n = newBinary(t.next(), n, t.F())
		default:
			return n
		}
	}
}

func (t *Tree) F() Node {
	switch token := t.peek(); token.typ {
	case itemNumber, itemFunc:
		return t.v()
	case itemNot, itemMinus:
		return newUnary(t.next(), t.O())
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
	case itemNumber:
		n, err := newNumber(token.pos, token.val)
		if err != nil {
			t.error(err)
		}
		return n
	case itemFunc:
		t.backup()
		return t.Func()
	default:
		t.unexpected(token, "input")
	}
	return nil
}

func (t *Tree) Func() (f *FuncNode) {
	token := t.next()
	funcv, ok := t.getFunction(token.val)
	if !ok {
		t.errorf("non existent function %s", token.val)
	}
	f = newFunc(token.pos, token.val, funcv)
	t.expect(itemLeftParen, "func")
	for {
		switch token = t.next(); token.typ {
		case itemNumber:
			n, err := newNumber(token.pos, token.val)
			if err != nil {
				t.error(err)
			}
			f.append(n)
		case itemString:
			s, err := strconv.Unquote(token.val)
			if err != nil {
				t.error(err)
			}
			f.append(newString(token.pos, token.val, s))
		case itemQuery:
			s := token.val[1 : len(token.val)-1]
			f.append(newQuery(token.pos, token.val, s))
		default:
			t.unexpected(token, "func")
		}
		switch token = t.next(); token.typ {
		case itemComma:
			// continue
		case itemRightParen:
			return
		default:
			t.unexpected(token, "func")
		}
	}
}

// hasFunction reports if a function name exists in the Tree's maps.
func (t *Tree) getFunction(name string) (v Func, ok bool) {
	for _, funcMap := range t.funcs {
		if funcMap == nil {
			continue
		}
		if v, ok = funcMap[name]; ok {
			return
		}
	}
	return
}
