// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scanner

import (
	"testing"
)

func TestMatchers(t *testing.T) {
	// Just basic checks, not exhaustive at all.
	checkMatch := func(s string, ttList ...interface{}) {
		scanner := New(s)

		i := 0
		for i < len(ttList) {
			tt := ttList[i].(tokenType)
			tVal := ttList[i+1].(string)
			if tok := scanner.Next(); tok.Type != tt || tok.Value != tVal {
				t.Errorf("did not match: %s (got %v)", s, tok)
			}

			i += 2
		}

		if tok := scanner.Next(); tok.Type != TokenEOF {
			t.Errorf("missing EOF after token %s, got %+v", s, tok)
		}
	}

	checkMatch("abcd", TokenIdent, "abcd")
	checkMatch(`"abcd"`, TokenString, `"abcd"`)
	checkMatch("'abcd'", TokenString, "'abcd'")
	checkMatch("#name", TokenHash, "#name")
	checkMatch("42''", TokenNumber, "42", TokenString, "''")
	checkMatch("4.2", TokenNumber, "4.2")
	checkMatch(".42", TokenNumber, ".42")
	checkMatch("42%", TokenPercentage, "42%")
	checkMatch("4.2%", TokenPercentage, "4.2%")
	checkMatch(".42%", TokenPercentage, ".42%")
	checkMatch("42px", TokenDimension, "42px")
	checkMatch("url('http://www.google.com/')", TokenURI, "url('http://www.google.com/')")
	checkMatch("U+0042", TokenUnicodeRange, "U+0042")
	checkMatch("<!--", TokenCDO, "<!--")
	checkMatch("-->", TokenCDC, "-->")
	checkMatch("   \n   \t   \n", TokenS, "   \n   \t   \n")
	checkMatch("/* foo */", TokenComment, "/* foo */")
	checkMatch("bar(", TokenFunction, "bar(")
	checkMatch("~=", TokenIncludes, "~=")
	checkMatch("|=", TokenDashMatch, "|=")
	checkMatch("^=", TokenPrefixMatch, "^=")
	checkMatch("$=", TokenSuffixMatch, "$=")
	checkMatch("*=", TokenSubstringMatch, "*=")
	checkMatch("{", TokenChar, "{")
	checkMatch("\uFEFF", TokenBOM, "\uFEFF")
	checkMatch(`╯︵┻━┻"stuff"`, TokenIdent, "╯︵┻━┻", TokenString, `"stuff"`)
}
