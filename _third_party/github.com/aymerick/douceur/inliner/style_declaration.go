package inliner

import "bosun.org/_third_party/github.com/aymerick/douceur/css"

type StyleDeclaration struct {
	StyleRule   *StyleRule
	Declaration *css.Declaration
}

func NewStyleDeclaration(styleRule *StyleRule, declaration *css.Declaration) *StyleDeclaration {
	return &StyleDeclaration{
		StyleRule:   styleRule,
		Declaration: declaration,
	}
}

// Computes style declaration specificity
func (styleDecl *StyleDeclaration) Specificity() int {
	if styleDecl.Declaration.Important {
		return 10000
	}

	return styleDecl.StyleRule.Specificity
}
