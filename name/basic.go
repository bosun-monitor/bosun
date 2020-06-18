package name

import "errors"

type basicValidationConfig struct {
	isEmptyValid     bool
	isValidRuneCheck func(rune) bool
}

// NewBasicValidator constructs a RuneLevelValidator which can validate names for being empty or containing isolated
// runes which fail a basic validation check
func NewBasicValidator(isEmptyValid bool, isValidRuneCheck func(rune) bool) (RuneLevelValidator, error) {
	if isValidRuneCheck == nil {
		return nil, errors.New("no isValidRuneCheck provided")
	}

	result := &basicValidationConfig{isEmptyValid: isEmptyValid, isValidRuneCheck: isValidRuneCheck}
	return result, nil
}

// IsValid returns true if the given name is valid according the objects validation checks
func (c *basicValidationConfig) IsValid(name string) bool {
	if len(name) == 0 {
		return c.isEmptyValid
	}

	for _, r := range name {
		if !c.isValidRuneCheck(r) {
			return false
		}
	}

	return true
}

// IsRuneValid returns true if the given rune is valid for use within a name
func (c *basicValidationConfig) IsRuneValid(r rune) bool {
	return c.isValidRuneCheck(r)
}
