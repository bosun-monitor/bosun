package host

import (
	"bosun.org/name"
	"errors"
	"fmt"
	"net"
	"strings"
)

const hostRegexPattern = `^([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])(\.([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9]))*$`

type hostNameFormatConfig struct {
	useFullName bool
	validators  []name.Validator
}

// NewHostNameProcessor constructs a new name.Processor for host names
func NewHostNameProcessor(useFullName bool) (name.Processor, error) {
	lenValidator := name.NewLengthValidator(1, 255)
	regexValidator, err := name.NewRegexpValidator(hostRegexPattern)
	if err != nil {
		return nil, err
	}

	result := &hostNameFormatConfig{
		useFullName: useFullName,
		validators:  []name.Validator{lenValidator, regexValidator},
	}

	return result, nil
}

// IsValid returns true if the provided name is valid according to https://tools.ietf.org/html/rfc1123
func (c *hostNameFormatConfig) IsValid(name string) bool {
	for _, validator := range c.validators {
		if !validator.IsValid(name) {
			return false
		}
	}

	return true
}

// FormatName takes a host name and formats it.
//
// If the name is an IP address then it's simply returned.
//
// If `useFullName == false` then the name is truncated at the first '.'.  If there is no '.' then the full name is
// returned.
//
// The resulting names will always be in lowercase format.
func (c *hostNameFormatConfig) FormatName(name string) (string, error) {
	if !c.useFullName {
		//only split if string is not an IP address
		ip := net.ParseIP(name)
		if ip == nil {
			name = strings.SplitN(name, ".", 2)[0]
		}
	}

	if !c.IsValid(name) {
		return "", errors.New(fmt.Sprintf("Invalid name of '%s'", name))
	}
	return strings.ToLower(name), nil
}
