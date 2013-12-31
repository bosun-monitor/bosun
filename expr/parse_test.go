// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package expr

import (
	"flag"
	"fmt"
	"strings"
	"testing"
)

var debug = flag.Bool("debug", false, "show the errors produced by the main tests")

type numberTest struct {
	text    string
	isInt   bool
	isUint  bool
	isFloat bool
	int64
	uint64
	float64
}

var numberTests = []numberTest{
	// basics
	{"0", true, true, true, 0, 0, 0},
	{"73", true, true, true, 73, 73, 73},
	{"073", true, true, true, 073, 073, 073},
	{"0x73", true, true, true, 0x73, 0x73, 0x73},
	{"100", true, true, true, 100, 100, 100},
	{"1e9", true, true, true, 1e9, 1e9, 1e9},
	{"1e19", false, true, true, 0, 1e19, 1e19},
	// funny bases
	{"0123", true, true, true, 0123, 0123, 0123},
	{"0xdeadbeef", true, true, true, 0xdeadbeef, 0xdeadbeef, 0xdeadbeef},
	// some broken syntax
	{text: "+-2"},
	{text: "0x123."},
	{text: "1e."},
	{text: "'x"},
	{text: "'xx'"},
}

func TestNumberParse(t *testing.T) {
	for _, test := range numberTests {
		n, err := newNumber(0, test.text)
		ok := test.isInt || test.isUint || test.isFloat
		if ok && err != nil {
			t.Errorf("unexpected error for %q: %s", test.text, err)
			continue
		}
		if !ok && err == nil {
			t.Errorf("expected error for %q", test.text)
			continue
		}
		if !ok {
			if *debug {
				fmt.Printf("%s\n\t%s\n", test.text, err)
			}
			continue
		}
		if test.isUint {
			if !n.IsUint {
				t.Errorf("expected unsigned integer for %q", test.text)
			}
			if n.Uint64 != test.uint64 {
				t.Errorf("uint64 for %q should be %d Is %d", test.text, test.uint64, n.Uint64)
			}
		} else if n.IsUint {
			t.Errorf("did not expect unsigned integer for %q", test.text)
		}
		if test.isFloat {
			if !n.IsFloat {
				t.Errorf("expected float for %q", test.text)
			}
			if n.Float64 != test.float64 {
				t.Errorf("float64 for %q should be %g Is %g", test.text, test.float64, n.Float64)
			}
		} else if n.IsFloat {
			t.Errorf("did not expect float for %q", test.text)
		}
	}
}

type parseTest struct {
	name   string
	input  string
	ok     bool
	result string // what the user would see in an error message.
}

const (
	noError  = true
	hasError = false
)

var parseTests = []parseTest{
	{"number", "1", noError, "1"},
	{"function", `avg(1, "abc", [test])`, noError, `avg(1, "abc", [test])`},
	// Errors.
	{"empty", "", hasError, ""},
	{"unclosed function", "avg(", hasError, ""},
	{"bad function", "bad(1)", hasError, ""},
}

var builtins = map[string]interface{}{
	"avg": fmt.Sprintf,
}

func TestParse(t *testing.T) {
	textFormat = "%q"
	defer func() { textFormat = "%s" }()
	for _, test := range parseTests {
		tmpl := New(test.name)
		err := tmpl.Parse(test.input, builtins)
		switch {
		case err == nil && !test.ok:
			t.Errorf("%q: expected error; got none", test.name)
			continue
		case err != nil && test.ok:
			t.Errorf("%q: unexpected error: %v", test.name, err)
			continue
		case err != nil && !test.ok:
			// expected error, got one
			if *debug {
				fmt.Printf("%s: %s\n\t%s\n", test.name, test.input, err)
			}
			continue
		}
		var result string
		result = tmpl.Root.String()
		if result != test.result {
			t.Errorf("%s=(%q): got\n\t%v\nexpected\n\t%v", test.name, test.input, result, test.result)
		}
	}
}

// All failures, and the result is a string that must appear in the error message.
var errorTests = []parseTest{
	// Check line numbers are accurate.
	{"unclosed1",
		"line1\n{{",
		hasError, `unclosed1:2: unexpected unclosed action in command`},
	{"unclosed2",
		"line1\n{{define `x`}}line2\n{{",
		hasError, `unclosed2:3: unexpected unclosed action in command`},
	// Specific errors.
	{"function",
		"{{foo}}",
		hasError, `function "foo" not defined`},
	{"comment",
		"{{/*}}",
		hasError, `unclosed comment`},
	{"lparen",
		"{{.X (1 2 3}}",
		hasError, `unclosed left paren`},
	{"rparen",
		"{{.X 1 2 3)}}",
		hasError, `unexpected ")"`},
	{"space",
		"{{`x`3}}",
		hasError, `missing space?`},
	{"idchar",
		"{{a#}}",
		hasError, `'#'`},
	{"charconst",
		"{{'a}}",
		hasError, `unterminated character constant`},
	{"stringconst",
		`{{"a}}`,
		hasError, `unterminated quoted string`},
	{"rawstringconst",
		"{{`a}}",
		hasError, `unterminated raw quoted string`},
	{"number",
		"{{0xi}}",
		hasError, `number syntax`},
	{"multidefine",
		"{{define `a`}}a{{end}}{{define `a`}}b{{end}}",
		hasError, `multiple definition of template`},
	{"eof",
		"{{range .X}}",
		hasError, `unexpected EOF`},
	{"variable",
		// Declare $x so it's defined, to avoid that error, and then check we don't parse a declaration.
		"{{$x := 23}}{{with $x.y := 3}}{{$x 23}}{{end}}",
		hasError, `unexpected ":="`},
	{"multidecl",
		"{{$a,$b,$c := 23}}",
		hasError, `too many declarations`},
	{"undefvar",
		"{{$a}}",
		hasError, `undefined variable`},
}

func _TestErrors(t *testing.T) {
	for _, test := range errorTests {
		err := New(test.name).Parse(test.input)
		if err == nil {
			t.Errorf("%q: expected error", test.name)
			continue
		}
		if !strings.Contains(err.Error(), test.result) {
			t.Errorf("%q: error %q does not contain %q", test.name, err, test.result)
		}
	}
}
