// Package 'name' contains contracts and functionality to help with naming things.
package name

// Validator is an interface which declares operations associated with name validation.
//
// IsValid returns true if the provided name is valid.
type Validator interface {
	IsValid(name string) bool
}

// RuneLevelValidator is Validator which can also validate isolated runes within a name.
//
// IsRuneValid returns true if the provided rune is a valid component of a name
type RuneLevelValidator interface {
	Validator
	IsRuneValid(r rune) bool
}

// Formatter is an interface which defines operations for types which can format names.
//
// FormatName takes a name, formats it and then returns the result.
type Formatter interface {
	FormatName(name string) (string, error)
}

// Processor is a Validator and Formatter.
type Processor interface {
	Validator
	Formatter
}

// RuneLevelProcessor is a RuneLevelValidator and Formatter.
type RuneLevelProcessor interface {
	RuneLevelValidator
	Formatter
}
