// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mof

import (
	"fmt"
	"strings"
	"unicode"
)

// item represents a token or text string returned from the scanner.
type item struct {
	typ itemType // The type of this item.
	pos pos      // The starting position, in bytes, of this item in the input string.
	val string   // The value of this item.
}

func (i item) String() string {
	switch {
	case i.typ == itemEOF:
		return "EOF"
	case i.typ == itemError:
		return i.val
	case len(i.val) > 10:
		return fmt.Sprintf("%.10q...", i.val)
	}
	return fmt.Sprintf("%q", i.val)
}

// itemType identifies the type of lex items.
type itemType int

const (
	itemError itemType = iota // error occurred; value is text of error
	itemEOF
	itemWord
	itemLeftBrace    // '{'
	itemRightBrace   // '}'
	itemLeftBracket  // '['
	itemRightBracket // ']'
	itemEq           // '='
	itemString
	itemNumber
	itemSemi // ';'
	itemComma
	itemChar
)

const eof = -1

// stateFn represents the state of the scanner as a function that returns the next state.
type stateFn func(*lexer) stateFn

// lexer holds the state of the scanner.
type lexer struct {
	input   []rune    // the string being scanned
	state   stateFn   // the next lexing function to enter
	pos     pos       // current position in the input
	start   pos       // start position of this item
	lastPos pos       // position of most recent item returned by nextItem
	items   chan item // channel of scanned items
}

// next returns the next rune in the input.
func (l *lexer) next() rune {
	if int(l.pos) >= len(l.input) {
		return eof
	}
	r := l.input[l.pos]
	l.pos++
	return r
}

// peek returns but does not consume the next rune in the input.
func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune. Can only be called once per call of next.
func (l *lexer) backup() {
	l.pos--
}

// emit passes an item back to the client.
func (l *lexer) emit(t itemType) {
	l.items <- item{t, l.start, string(l.input[l.start:l.pos])}
	l.start = l.pos
}

// accept consumes the next rune if it's from the valid set.
func (l *lexer) accept(valid string) bool {
	if strings.IndexRune(valid, l.next()) >= 0 {
		return true
	}
	l.backup()
	return false
}

// acceptRun consumes a run of runes from the valid set.
func (l *lexer) acceptRun(valid string) {
	for strings.IndexRune(valid, l.next()) >= 0 {
	}
	l.backup()
}

// ignore skips over the pending input before this point.
func (l *lexer) ignore() {
	l.start = l.pos
}

// lineNumber reports which line we're on, based on the position of
// the previous item returned by nextItem. Doing it this way
// means we don't have to worry about peek double counting.
func (l *lexer) lineNumber() int {
	return 1 + strings.Count(string(l.input[:l.lastPos]), "\n")
}

// errorf returns an error token and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.nextItem.
func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- item{itemError, l.start, fmt.Sprintf(format, args...)}
	return nil
}

// nextItem returns the next item from the input.
func (l *lexer) nextItem() item {
	item := <-l.items
	l.lastPos = item.pos
	return item
}

// lex creates a new scanner for the input string.
func lex(input []rune) *lexer {
	l := &lexer{
		input: input,
		items: make(chan item),
	}
	go l.run()
	return l
}

// run runs the state machine for the lexer.
func (l *lexer) run() {
	for l.state = lexItem; l.state != nil; {
		l.state = l.state(l)
	}
}

// state functions

func lexItem(l *lexer) stateFn {
Loop:
	for {
		switch r := l.next(); {
		case r == '{':
			l.emit(itemLeftBrace)
		case r == '}':
			l.emit(itemRightBrace)
		case r == '[':
			l.emit(itemLeftBracket)
		case r == ']':
			l.emit(itemRightBracket)
		case r == '=':
			l.emit(itemEq)
		case r == ';':
			l.emit(itemSemi)
		case r == ',':
			l.emit(itemComma)
		case isNumber(r):
			l.backup()
			return lexNumber
		case r == '"':
			return lexString
		case isSpace(r):
			l.ignore()
		case isWord(r):
			l.backup()
			return lexWord
		case r == eof:
			l.emit(itemEOF)
			break Loop
		default:
			l.emit(itemChar)
		}
	}
	return nil
}

// lexNumber scans a number: decimal, octal, hex, float, or imaginary. This
// isn't a perfect number scanner - for instance it accepts "." and "0x0.2"
// and "089" - but when it's wrong the input is invalid and the parser (via
// strconv) will notice.
func lexNumber(l *lexer) stateFn {
	if !l.scanNumber() {
		return l.errorf("bad number syntax: %q", l.input[l.start:l.pos])
	}
	l.emit(itemNumber)
	return lexItem
}

func (l *lexer) scanNumber() bool {
	// Is it hex?
	digits := "0123456789"
	if l.accept("0") && l.accept("xX") {
		digits = "0123456789abcdefABCDEF"
	}
	l.acceptRun(digits)
	if l.accept(".") {
		l.acceptRun(digits)
	}
	if l.accept("eE") {
		l.accept("+-")
		l.acceptRun("0123456789")
	}
	return true
}

func lexWord(l *lexer) stateFn {
	first := true
	for {
		r := l.next()
		switch {
		case isWord(r):
			// absorb
		case unicode.IsNumber(r) && !first:
			// absorb
		default:
			l.backup()
			l.emit(itemWord)
			return lexItem
		}
		first = false
	}
}

func lexString(l *lexer) stateFn {
	var prev rune
	for {
		c := l.next()
		switch {
		case c == '"' && prev != '\\':
			l.emit(itemString)
			return lexItem
		case c == eof:
			return l.errorf("unterminated string")
		case prev == '\\' && c == '\\':
			prev = 0
		default:
			prev = c
		}
	}
}

// isSpace reports whether r is a space character.
func isSpace(r rune) bool {
	return unicode.IsSpace(r)
}

func isNumber(r rune) bool {
	return unicode.IsDigit(r)
}

func isWord(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}
