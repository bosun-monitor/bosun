package opentsdb

import (
	"fmt"
	"strings"
	"unicode"

	"bosun.org/name"
	"github.com/pkg/errors"
)

type openTsdbNameConfig struct {
	invalidRuneReplacement string
	basicValidator         name.RuneLevelValidator
}

// NewOpenTsdbNameProcessor constructs a new name.RuneLevelProcessor which can work with the OpenTSDB name format
func NewOpenTsdbNameProcessor(invalidRuneReplacement string) (name.RuneLevelProcessor, error) {
	bv, err := name.NewBasicValidator(false, func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' || r == '/'
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to construct basic validator")
	}

	result := &openTsdbNameConfig{invalidRuneReplacement: invalidRuneReplacement, basicValidator: bv}

	return result, nil
}

// IsRuneValid returns true if an isolated rune is valid within an OpenTSDB format name
func (c *openTsdbNameConfig) IsRuneValid(r rune) bool {
	return c.basicValidator.IsRuneValid(r)
}

// IsValid returns true if a name is valid according the OpenTSDB rules
func (c *openTsdbNameConfig) IsValid(name string) bool {
	return c.basicValidator.IsValid(name)
}

// FormatName takes a name and attempts to ensure that is valid according to OpenTSDB rules.  If a names contains
// any invalid runes then these are replaced with 'invalidRuneReplacement'.
func (c *openTsdbNameConfig) FormatName(name string) (string, error) {
	sb := strings.Builder{}
	var err error
	var lastRune rune
	for _, r := range name {
		if c.IsRuneValid(r) {
			_, err = sb.WriteRune(r)
		} else if lastRune != r { // don't do a 1 for 1 replacement on a run of invalid runes
			_, err = sb.WriteString(c.invalidRuneReplacement)
		}

		if err != nil {
			return "", errors.Wrap(err, fmt.Sprintf("Failed to format '%s'", name))
		}

		lastRune = r
	}

	result := sb.String()
	if result == "" {
		return "", errors.New("Name left empty after formatting")
	}

	return result, nil
}
