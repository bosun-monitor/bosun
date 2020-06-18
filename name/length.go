package name

type lengthValidationConfig struct {
	minLen uint
	maxLen uint
}

// NewLengthValidator constructs a Validator which can verify that names are of a correct length
func NewLengthValidator(min uint, max uint) Validator {
	return &lengthValidationConfig{minLen: min, maxLen: max}
}

// IsValid returns true if the name if of an acceptable length
func (c *lengthValidationConfig) IsValid(name string) bool {
	length := uint(len(name))

	return length >= c.minLen && length <= c.maxLen
}
