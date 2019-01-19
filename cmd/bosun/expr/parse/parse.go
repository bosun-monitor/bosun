// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package parse builds parse trees for expressions as defined by expr. Clients
// should use that package to construct expressions rather than this one, which
// provides shared internal data structures not intended for general use.
package parse // import "bosun.org/cmd/bosun/expr/parse"

import (
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"bosun.org/models"
)

// Tree is the representation of a single parsed expression.
type Tree struct {
	Text string // text parsed to create the expression.
	Root Node   // top-level root of the tree, returns a number.

	funcs   []map[string]Func
	mapExpr bool

	// Parsing only; cleared after parse.
	lex       *lexer
	token     [1]item // one-token lookahead for parser.
	peekCount int
}

// returnTagsFunc is the type for functions that return the expected tag keys
// for an expression function
type returnTagsFunc func([]Node) (TagKeys, error)

// Func is used to represent an Function within Bosun's expression language and has
// the properties of the function needed to check validity of the expression.
type Func struct {
	Args    []models.FuncType // an array of the the expected types to the function argument.
	Return  models.FuncType   // expected return type of the function.
	TagKeys returnTagsFunc    // function that returns the expected tag keys for the result of the function.
	F       interface{}       // a pointer to the actual Go code that will execute.

	VArgs     bool // if this function support variable length arguments.
	VArgsPos  int  // what argument index variable length arguments start at.
	VArgsOmit bool // if in a variable argument functions the variable arguments may be ommited entirely.

	// if the function is a generic function that will return a seriesSet or NumberSet depending
	// on what type was passed to it.
	VariantReturn bool

	MapFunc bool // indiciates if the function is only valid within a map expression.

	// if this function supports "Prefix Notation", for example ["foo"]myFunc. When this is the
	// case the first argument passed to the F function will be the string in the prefix.
	PrefixEnabled bool
	// a property that is set to indicate the presence of the optional prefix.
	PrefixKey bool
	// an optional function, that if defined will be used to check the validy of the function at parse time
	// before the function is ever called.
	Check func(*Tree, *FuncNode) error
}

// Tag Keys is a set of keys
type TagKeys map[string]struct{}

// String represents the set of tag keys as a CSV
func (t TagKeys) String() string {
	var keys []string
	for k := range t {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

// Equal returns if o is has equal tag keys
func (t TagKeys) Equal(o TagKeys) bool {
	if len(t) != len(o) {
		return false
	}
	for k := range t {
		if _, present := o[k]; !present {
			return false
		}
	}
	return true
}

// Subset returns true if o is a subset of t.
func (t TagKeys) Subset(o TagKeys) bool {
	for k := range t {
		if _, ok := o[k]; !ok {
			return false
		}
	}
	return true
}

// Intersection returns Tags common to both tagsets.
func (t TagKeys) Intersection(o TagKeys) TagKeys {
	result := TagKeys{}
	for k := range t {
		if _, ok := o[k]; ok {
			result[k] = struct{}{}
		}
	}
	return result
}

// Parse returns a Tree, created by parsing the expression described in the
// argument string. If an error is encountered, parsing stops and an empty Tree
// is returned with the error.
func Parse(text string, funcs ...map[string]Func) (t *Tree, err error) {
	t = New()
	t.Text = text
	err = t.Parse(text, funcs...)
	return
}

func ParseSub(text string, funcs ...map[string]Func) (t *Tree, err error) {
	t = New()
	t.mapExpr = true
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
	for _, funcMap := range funcs {
		for name, f := range funcMap {
			if f.VariantReturn {
				if f.TagKeys == nil {
					panic(fmt.Errorf("%v: expected Tags definition: got nil", name))
				}
				continue
			}
			switch f.Return {
			case models.TypeSeriesSet, models.TypeNumberSet:
				if f.TagKeys == nil {
					panic(fmt.Errorf("%v: expected Tags definition: got nil", name))
				}
			default:
				if f.TagKeys != nil {
					panic(fmt.Errorf("%v: unexpected Tags definition: expected nil", name))
				}
			}
		}
	}
}

// stopParse terminates parsing.
func (t *Tree) stopParse() {
	t.lex = nil
}

// Parse parses the expression definition string to construct a representation
// of the expression for execution.
func (t *Tree) Parse(text string, funcs ...map[string]Func) (err error) {
	defer t.recover(&err)
	t.startParse(funcs, lex(text))
	t.Text = text
	t.parse()
	t.stopParse()
	return nil
}

// parse is the top-level parser for an expression.
// It runs to EOF.
func (t *Tree) parse() {
	t.Root = t.O()
	t.expect(itemEOF, "root input")
	if err := t.Root.Check(t); err != nil {
		t.error(err)
	}
}

/* Grammar:
O -> A {"||" A}
A -> C {"&&" C}
C -> P {( "==" | "!=" | ">" | ">=" | "<" | "<=") P}
P -> M {( "+" | "-" ) M}
M -> E {( "*" | "/" ) F}
E -> F {( "**" ) F}
F -> v | "(" O ")" | "!" O | "-" O
v -> number | func(..)
Func -> optPrefix name "(" param {"," param} ")"
param -> number | "string" | subExpr | [query]
optPrefix -> [ prefix ]
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
	n := t.E()
	for {
		switch t.peek().typ {
		case itemMult, itemDiv, itemMod:
			n = newBinary(t.next(), n, t.E())
		default:
			return n
		}
	}
}

func (t *Tree) E() Node {
	n := t.F()
	for {
		switch t.peek().typ {
		case itemPow:
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
		return newUnary(t.next(), t.F())
	case itemPrefix:
		token := t.next()
		return newPrefix(token.val, token.pos, t.F())
	case itemLeftParen:
		t.next()
		n := t.O()
		t.expect(itemRightParen, "input: F()")
		return n
	default:
		t.unexpected(token, "input: F()")
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
		t.unexpected(token, "input: v()")
	}
	return nil
}

// Func parses a FuncNode
func (t *Tree) Func() (f *FuncNode) {
	token := t.next()
	funcv, ok := t.GetFunction(token.val)
	if !ok {
		t.errorf("non existent function %s", token.val)
	}
	f = newFunc(token.pos, token.val, funcv)
	t.expect(itemLeftParen, "func")
	for {
		switch token = t.next(); token.typ {
		default:
			t.backup()
			node := t.O()
			f.append(node)
			if len(f.Args) == 1 && f.F.VariantReturn {
				f.F.Return = node.Return()
			}
		case itemTripleQuotedString:
			f.append(newString(token.pos, token.val, token.val[3:len(token.val)-3]))
		case itemString:
			s, err := strconv.Unquote(token.val)
			if err != nil {
				t.errorf("Unquoting error: %s", err)
			}
			f.append(newString(token.pos, token.val, s))
		case itemRightParen:
			return
		case itemExpr:
			t.expect(itemLeftParen, "v() expect left paran in itemExpr")
			start := t.lex.lastPos
			leftCount := 1
		TOKENS:
			for {
				switch token = t.next(); token.typ {
				case itemLeftParen:
					leftCount++
				case itemFunc:
				case itemRightParen:
					leftCount--
					if leftCount == 0 {
						t.expect(itemRightParen, "v() expect right paren in itemExpr")
						t.backup()
						break TOKENS
					}
				case itemEOF:
					t.unexpected(token, "input: v()")
				default:
					// continue
				}
			}
			n, err := newExprNode(t.lex.input[start:t.lex.lastPos], t.lex.lastPos)
			if err != nil {
				t.error(err)
			}
			n.Tree, err = ParseSub(n.Text, t.funcs...)
			if err != nil {
				t.error(err)
			}
			f.append(n)
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

// GetFunction returns the Func specification for given function name
func (t *Tree) GetFunction(name string) (v Func, ok bool) {
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

func (t *Tree) String() string {
	return t.Root.String()
}
