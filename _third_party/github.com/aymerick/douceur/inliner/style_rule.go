package inliner

import (
	"fmt"
	"regexp"
	"strings"

	"bosun.org/_third_party/github.com/aymerick/douceur/css"
)

const (
	INLINE_FAKE_SELECTOR = "*INLINE*"

	// Regular expressions borrowed from premailer:
	//   https://github.com/premailer/css_parser/blob/master/lib/css_parser/regexps.rb
	NON_ID_ATTRIBUTES_AND_PSEUDO_CLASSES_REGEXP = `(?i)(\.[\w]+)|\[(\w+)|(\:(link|visited|active|hover|focus|lang|target|enabled|disabled|checked|indeterminate|root|nth-child|nth-last-child|nth-of-type|nth-last-of-type|first-child|last-child|first-of-type|last-of-type|only-child|only-of-type|empty|contains))`
	ELEMENTS_AND_PSEUDO_ELEMENTS_REGEXP         = `(?i)((^|[\s\+\>\~]+)[\w]+|\:{1,2}(after|before|first-letter|first-line|selection))`
)

var (
	nonIDAttrAndPseudoClassesRegexp *regexp.Regexp
	elementsAndPseudoElementsRegexp *regexp.Regexp
)

// A Qualifier Rule for a uniq selector
type StyleRule struct {
	// The style rule selector
	Selector string

	// The style rule properties
	Declarations []*css.Declaration

	// Selector specificity
	Specificity int
}

func init() {
	nonIDAttrAndPseudoClassesRegexp, _ = regexp.Compile(NON_ID_ATTRIBUTES_AND_PSEUDO_CLASSES_REGEXP)
	elementsAndPseudoElementsRegexp, _ = regexp.Compile(ELEMENTS_AND_PSEUDO_ELEMENTS_REGEXP)
}

// Instanciate a new StyleRule
func NewStyleRule(selector string, declarations []*css.Declaration) *StyleRule {
	return &StyleRule{
		Selector:     selector,
		Declarations: declarations,
		Specificity:  ComputeSpecificity(selector),
	}
}

// Returns the string representation of a style rule
func (styleRule *StyleRule) String() string {
	result := ""

	result += styleRule.Selector

	if len(styleRule.Declarations) == 0 {
		result += ";"
	} else {
		result += " {\n"

		for _, decl := range styleRule.Declarations {
			result += fmt.Sprintf("  %s\n", decl.String())
		}

		result += "}"
	}

	return result
}

// Computes style rule specificity
//
// cf. http://www.w3.org/TR/selectors/#specificity
func ComputeSpecificity(selector string) int {
	result := 0

	if selector == INLINE_FAKE_SELECTOR {
		result += 1000
	}

	result += 100 * strings.Count(selector, "#")
	result += 10 * len(nonIDAttrAndPseudoClassesRegexp.FindAllStringSubmatch(selector, -1))
	result += len(elementsAndPseudoElementsRegexp.FindAllStringSubmatch(selector, -1))

	return result
}
