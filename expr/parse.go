// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package parse builds parse trees for templates as defined by text/template
// and html/template. Clients should use those packages to construct templates
// rather than this one, which provides shared internal data structures not
// intended for general use.
package expr

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

// Tree is the representation of a single parsed template.
type Tree struct {
	Name      string    // name of the template represented by the tree.
	ParseName string    // name of the top-level template during parsing, for error messages.
	Root      *BoolNode // top-level root of the tree.
	text      string    // text parsed to create the template (or its parent)
	// Parsing only; cleared after parse.
	funcs     []map[string]interface{}
	lex       *lexer
	token     [1]item // one-token lookahead for parser.
	peekCount int
}

// Parse returns a map from template name to parse.Tree, created by parsing the
// templates described in the argument string. The top-level template will be
// given the specified name. If an error is encountered, parsing stops and an
// empty map is returned with the error.
func Parse(name, text string, funcs ...map[string]interface{}) (t *Tree, err error) {
	t = New(name)
	t.text = text
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
func New(name string, funcs ...map[string]interface{}) *Tree {
	return &Tree{
		Name:  name,
		funcs: funcs,
	}
}

// ErrorContext returns a textual representation of the location of the node in the input text.
func (t *Tree) ErrorContext(n Node) (location, context string) {
	pos := int(n.Position())
	text := t.text[:pos]
	byteNum := strings.LastIndex(text, "\n")
	if byteNum == -1 {
		byteNum = pos // On first line.
	} else {
		byteNum++ // After the newline.
		byteNum = pos - byteNum
	}
	lineNum := 1 + strings.Count(text, "\n")
	context = n.String()
	if len(context) > 20 {
		context = fmt.Sprintf("%.20s...", context)
	}
	return fmt.Sprintf("%s:%d:%d", t.ParseName, lineNum, byteNum), context
}

// errorf formats the error and terminates processing.
func (t *Tree) errorf(format string, args ...interface{}) {
	t.Root = nil
	format = fmt.Sprintf("template: %s:%d: %s", t.ParseName, t.lex.lineNumber(), format)
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
func (t *Tree) startParse(funcs []map[string]interface{}, lex *lexer) {
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
func (t *Tree) Parse(text string, funcs ...map[string]interface{}) (err error) {
	defer t.recover(&err)
	t.ParseName = t.Name
	t.startParse(funcs, lex(t.Name, text))
	t.text = text
	t.parse()
	t.stopParse()
	return nil
}

// parse is the top-level parser for a template.
// It runs to EOF.
func (t *Tree) parse() {
	t.Root = newBool(t.peek().pos)
	t.Root.Expr = t.expr()
	if p := t.peek(); p.typ != itemEOF {
		t.errorf("unexpected %s", p)
	}
}

/* Grammar:
O -> A {"||" A}
A -> C {"&&" C}
C -> P {( "==" | "!=" | ">" | ">=" | "<" | "<=") P}
P -> M {( "+" | "-" ) M}
M -> P {( "*" | "/" ) P}
P -> v | "(" O ")" | "!" O | "-" O
v -> number | func(..)
param -> number | "string" | [query]
func -> name "(" param {"," param} ")"
*/

// expr:
func (t *Tree) expr() Node {
	v := t.value()
	return v
}

func (t *Tree) value() Node {
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
	if !t.hasFunction(token.val) {
		t.errorf("non existent function %s", token.val)
	}
	f = newFunc(token.pos, token.val)
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
func (t *Tree) hasFunction(name string) bool {
	for _, funcMap := range t.funcs {
		if funcMap == nil {
			continue
		}
		if funcMap[name] != nil {
			return true
		}
	}
	return false
}
