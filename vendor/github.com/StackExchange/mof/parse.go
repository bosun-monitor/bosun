// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mof

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Tree is the representation of a single parsed configuration.
type tree struct {
	Root *classNode // top-level root of the tree.
	text []byte     // text parsed to create the template (or its parent)
	// Parsing only; cleared after parse.
	lex       *lexer
	token     [1]item // one-token lookahead for parser.
	peekCount int
}

// Parse returns a Tree, created by parsing the configuration described in the
// argument string. If an error is encountered, parsing stops and an empty Tree
// is returned with the error.
func parse(text []byte) (t *tree, err error) {
	t = &tree{
		text: text,
	}
	err = t.Parse(text)
	return
}

// next returns the next token.
func (t *tree) next() item {
	if t.peekCount > 0 {
		t.peekCount--
	} else {
		t.token[0] = t.lex.nextItem()
	}
	return t.token[t.peekCount]
}

// backup backs the input stream up one token.
func (t *tree) backup() {
	t.peekCount++
}

// peek returns but does not consume the next token.
func (t *tree) peek() item {
	if t.peekCount > 0 {
		return t.token[t.peekCount-1]
	}
	t.peekCount = 1
	t.token[0] = t.lex.nextItem()
	return t.token[0]
}

// Parsing.

// errorf formats the error and terminates processing.
func (t *tree) errorf(format string, args ...interface{}) {
	t.Root = nil
	format = fmt.Sprintf("parse: %d: %s", t.lex.lineNumber(), format)
	panic(fmt.Errorf(format, args...))
}

// error terminates processing.
func (t *tree) error(err error) {
	t.errorf("%s", err)
}

// expect consumes the next token and guarantees it has the required type.
func (t *tree) expect(expected itemType, context string) item {
	token := t.next()
	if token.typ != expected {
		t.unexpected(token, context)
	}
	return token
}

// expectOneOf consumes the next token and guarantees it has one of the required types.
func (t *tree) expectOneOf(expected1, expected2 itemType, context string) item {
	token := t.next()
	if token.typ != expected1 && token.typ != expected2 {
		t.unexpected(token, context)
	}
	return token
}

// until ignores tokens until a specified type is found.
func (t *tree) until(until itemType, context string) item {
	for {
		token := t.next()
		switch token.typ {
		case until:
			return token
		case itemEOF:
			t.errorf("did not find %s in %s", until, context)
		}
	}
}

func (t *tree) expectWord(val string, context string) item {
	token := t.next()
	if token.typ != itemWord {
		t.unexpected(token, context)
	}
	if token.val != val {
		t.errorf("expected word %s in %s", val, context)
	}
	return token
}

// unexpected complains about the token and terminates processing.
func (t *tree) unexpected(token item, context string) {
	t.errorf("unexpected %s in %s", token, context)
}

// recover is the handler that turns panics into returns from the top level of Parse.
func (t *tree) recover(errp *error) {
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
func (t *tree) startParse(lex *lexer) {
	t.Root = nil
	t.lex = lex
}

// stopParse terminates parsing.
func (t *tree) stopParse() {
	t.lex = nil
}

// Parse parses the template definition string to construct a representation of
// the template for execution. If either action delimiter string is empty, the
// default ("{{" or "}}") is used. Embedded template definitions are added to
// the treeSet map.
func (t *tree) Parse(text []byte) (err error) {
	defer t.recover(&err)
	runes, err := toRunes(text)
	if err != nil {
		return err
	}
	t.startParse(lex(runes))
	t.text = text
	t.Root = newClass(t.peek().pos)
	t.parse(t.Root)
	t.stopParse()
	return nil
}

func toRunes(text []byte) ([]rune, error) {
	var runes []rune
	if bytes.HasPrefix(text, []byte{0, 0, 0xFE, 0xFF}) {
		return nil, fmt.Errorf("mof: unsupported encoding: UTF-32, big-endian")
	} else if bytes.HasPrefix(text, []byte{0xFF, 0xFE, 0, 0}) {
		return nil, fmt.Errorf("mof: unsupported encoding: UTF-32, little-endian")
	} else if bytes.HasPrefix(text, []byte{0xFE, 0xFF}) {
		return nil, fmt.Errorf("mof: unsupported encoding: UTF-16, big-endian")
	} else if bytes.HasPrefix(text, []byte{0xFF, 0xFE}) {
		// UTF-16, little-endian
		for len(text) > 0 {
			u := binary.LittleEndian.Uint16(text)
			runes = append(runes, rune(u))
			text = text[2:]
		}
		return runes, nil
	}
	// Assume UTF-8.
	for len(text) > 0 {
		r, s := utf8.DecodeRune(text)
		if r == utf8.RuneError {
			return nil, fmt.Errorf("mof: unrecognized encoding")
		}
		runes = append(runes, r)
		text = text[s:]
	}
	return runes, nil
}

// parse is the top-level parser for a conf.
// It runs to EOF.
func (t *tree) parse(n *classNode) {
	foundClass := false
	for {
		token := t.next()
		switch token.typ {
		case itemWord:
			if token.val == "class" {
				if foundClass {
					t.unexpected(token, "input")
				}
				foundClass = true
				t.parseClass(n)
				t.expect(itemSemi, "input")
			}
		case itemEOF:
			if foundClass {
				return
			}
			t.unexpected(token, "input")
		default:
			// ignore
		}
	}
}

func (t *tree) parseClass(n *classNode) {
	const context = "class"
	token := t.expect(itemWord, context)
	if token.val != "__PARAMETERS" {
		t.errorf("expected __PARAMETERS")
	}
	t.expect(itemLeftBrace, context)
	var brackets int
	var btwn []string
	for {
		token := t.next()
		// Ignore everything between braces.
		if brackets > 0 && token.typ != itemLeftBracket && token.typ != itemRightBracket {
			btwn = append(btwn, token.val)
			continue
		}
		switch token.typ {
		case itemLeftBracket:
			if brackets == 0 {
				btwn = nil
			}
			brackets++
		case itemRightBracket:
			brackets--
		case itemWord:
			// If [out] was just parsed, then the type is next, not the name.
			if strings.Join(btwn, " ") == "out" {
				token = t.expect(itemWord, context)
			}
			btwn = nil
			member := token.val
			t.until(itemEq, context)
			n.Members[member] = t.parseValue()
			t.expect(itemSemi, context)
		case itemRightBrace:
			return
		default:
			t.unexpected(token, context)
		}
	}
}

func (t *tree) parseValue() node {
	const context = "value"
	token := t.next()
	var n node
	switch token.typ {
	case itemLeftBrace:
		t.backup()
		n = t.parseArray()
	case itemString:
		s, err := strconv.Unquote(token.val)
		if err != nil {
			t.error(err)
		}
		n = newString(token.pos, token.val, s)
	case itemNumber:
		var err error
		n, err = newNumber(token.pos, token.val)
		if err != nil {
			t.error(err)
		}
	case itemWord:
		switch token.val {
		case "TRUE":
			n = newBool(token.pos, true)
		case "FALSE":
			n = newBool(token.pos, false)
		case "instance":
			t.backup()
			n = t.parseInstance()
		case "NULL":
			n = newNil(token.pos)
		default:
			t.unexpected(token, context)
		}
	default:
		t.unexpected(token, context)
	}
	return n
}

func (t *tree) parseInstance() *instanceNode {
	const context = "instance"
	token := t.expectWord("instance", context)
	t.expectWord("of", context)
	t.expect(itemWord, context)
	t.expect(itemLeftBrace, context)
	i := newInstance(token.pos)
	for {
		token := t.next()
		switch token.typ {
		case itemWord:
			member := token.val
			t.until(itemEq, context)
			i.Members[member] = t.parseValue()
			t.expect(itemSemi, context)
		case itemRightBrace:
			return i
		default:
			t.unexpected(token, context)
		}
	}
}

func (t *tree) parseArray() *arrayNode {
	const context = "array"
	token := t.expect(itemLeftBrace, context)
	a := newArray(token.pos)
Loop:
	for {
		token = t.next()
		switch token.typ {
		case itemRightBrace:
			break Loop
		default:
			t.backup()
		}
		a.Array = append(a.Array, t.parseValue())
		if a.Array[0].Type() != a.Array[len(a.Array)-1].Type() {
			t.errorf("unmatching types in array")
		}
		token = t.next()
		switch token.typ {
		case itemComma:
			// continue
		case itemRightBrace:
			break Loop
		default:
			t.unexpected(token, context)
		}
	}
	return a
}
