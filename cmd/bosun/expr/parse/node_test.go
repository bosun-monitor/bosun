package parse

import (
	"testing"
)

type nodeTest struct {
	name   string
	input  string
	result string
}

var nodeTests = []nodeTest{
	// Test that expressions are serialised with the necessary
	// parentheses. Since binary expressions need to be aware of
	// the relative precedence of child expressions, we test trees
	// where the root operator is of equal precedence, one level
	// higher, and one level lower.

	// Lowest precedence: ||
	{"binary parens || over ||", "(0 || 1) || (1 || 0)", "0 || 1 || (1 || 0)"},
	{"binary parens || over &&", "(0 && 1) || (1 && 0)", "0 && 1 || 1 && 0"},
	{"binary parens && over ||", "(0 || 1) && (1 || 0)", "(0 || 1) && (1 || 0)"},
	// Higher precedence: &&
	{"binary parens && over &&", "(0 && 1) && (1 && 0)", "0 && 1 && (1 && 0)"},
	{"binary parens && over >=", "(0 >= 1) && (1 >= 0)", "0 >= 1 && 1 >= 0"},
	{"binary parens >= over &&", "(0 && 1) >= (1 && 0)", "(0 && 1) >= (1 && 0)"},
	// Higher precedence: >=
	{"binary parens >= over >=", "(0 >= 1) >= (1 >= 0)", "0 >= 1 >= (1 >= 0)"},
	{"binary parens >= over +", "(0 + 1) >= (1 + 0)", "0 + 1 >= 1 + 0"},
	{"binary parens + over >=", "(0 >= 1) + (1 >= 0)", "(0 >= 1) + (1 >= 0)"},
	// Higher precedence: +
	{"binary parens + over +", "(0 + 1) + (1 + 0)", "0 + 1 + (1 + 0)"},
	{"binary parens + over *", "(0 * 1) + (1 * 0)", "0 * 1 + 1 * 0"},
	{"binary parens * over +", "(0 + 1) * (1 + 0)", "(0 + 1) * (1 + 0)"},
	// Higher precedence: **
	{"binary parens + over +", "(0 + 1) + (1 + 0)", "0 + 1 + (1 + 0)"},
	{"binary parens + over **", "(0 ** 1) + (1 ** 0)", "0 ** 1 + 1 ** 0"},
	{"binary parens ** over +", "(0 + 1) ** (1 + 0)", "(0 + 1) ** (1 + 0)"},

	{"unary parens ! around binary op", "!(0 && 1)", "!(0 && 1)"},
	{"unary parens ! alone", "!(0)", "!0"},

	{"unary parens - around binary op", "-(1 ** 1)", "-(1 ** 1)"},
	{"unary parens - alone", "-(1)", "-1"},

	// This might be a surprise! Some languages make exponentiation higher
	// precedence than negation, but our unary operators always take
	// higher precedence than exponentiation, so .String() will not
	// reproduce these redundant parens.
	{"unary parens - inside **", "(-1) ** 1", "-1 ** 1"},
}

func TestNodeString(t *testing.T) {
	textFormat = "%q"
	defer func() { textFormat = "%s" }()
	for _, test := range nodeTests {
		tmpl := New(nil)
		err := tmpl.Parse(test.input, builtins)
		if err != nil {
			t.Errorf("%q: unexpected error: %v", test.name, err)
			continue
		} else {
			var result string
			result = tmpl.Root.String()
			if result != test.result {
				t.Errorf("%s=(%q): got\n\t%v\nexpected\n\t%v", test.name, test.input, result, test.result)
			}
		}
	}
}
