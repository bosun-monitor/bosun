// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// item represents a token or text string returned from the scanner.
type item struct {
	typ itemType // The type of this item.
	pos Pos      // The starting position, in bytes, of this item in the input string.
	val string   // The value of this item.
}

type itemType int

const (
	itemError itemType = iota // error occurred; value is text of error
	itemEOF

	// Literals
	itemAsk // field:query

	itemNot        // '!'
	itemAnd        // AND
	itemOr         // OR
	itemLeftParen  // (
	itemRightParen // )
)

const eof = -1

// stateFn represents the state of the scanner as a function that returns the next state.
type stateFn func(*lexer) stateFn

// lexer holds the state of the scanner.
type lexer struct {
	input   string    // the string being scanned
	state   stateFn   // the next lexing function to enter
	pos     Pos       // current position in the input
	start   Pos       // start position of this item
	width   Pos       // width of last rune read from input
	lastPos Pos       // position of most recent item returned by nextItem
	items   chan item // channel of scanned items
}

// next returns the next rune in the input.
func (l *lexer) next() rune {
	if int(l.pos) >= len(l.input) {
		l.width = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.width = Pos(w)
	l.pos += l.width
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
	l.pos -= l.width
}

// emit passes an item back to the client.
func (l *lexer) emit(t itemType) {
	l.items <- item{t, l.start, l.input[l.start:l.pos]}
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
	return 1 + strings.Count(l.input[:l.lastPos], "\n")
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
func lex(input string) *lexer {
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
		case unicode.IsLetter(r):
			return lexWord
		case r == '(':
			l.emit(itemLeftParen)
		case r == ')':
			l.emit(itemRightParen)
		case r == '!':
			l.emit(itemNot)
		case isSpace(r):
			l.ignore()
		case r == eof:
			l.emit(itemEOF)
			break Loop
		default:
			return l.errorf("invalid character: %s", string(r))
		}
	}
	return nil
}

func lexWord(l *lexer) stateFn {
	for {
		switch r := l.next(); {
		case !isSpace(r) && r != eof && r != ')':
			// absorb
		default:
			l.backup()
			s := l.input[l.start:l.pos]
			switch s {
			case "AND":
				l.emit(itemAnd)
			case "OR":
				l.emit(itemOr)
			default:
				l.emit(itemAsk)
			}
			return lexItem
		}
	}
}

// isSpace reports whether r is a space character.
func isSpace(r rune) bool {
	return unicode.IsSpace(r)
}
