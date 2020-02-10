// Package host contains functionality for dealing with the machine which Bosun is running on.
package host

import (
	"bosun.org/name"
	"fmt"
	"github.com/pkg/errors"
)

type host struct {
	nameProcessor name.Processor
	providedName  string
	workingName   string
}

// Host is an interface which defines operations which can be performed against the machine which Bosun is running on.
//
// GetName returns the name of the host.
//
// SetName allows for the current host name to be overridden.
//
// GetNameProcessor returns the name.Processor that is associated with the host.
//
// SetNameProcessor allows for the name.Processor that is associated with the host to be overridden.
type Host interface {
	GetName() string
	SetName(name string) error
	GetNameProcessor() name.Processor
	SetNameProcessor(np name.Processor) error
}

// NewHost constructs a new Host object
func NewHost(name string, np name.Processor) (Host, error) {
	if name == "" {
		return nil, errors.New("No name provided")
	}

	if np == nil {
		return nil, errors.New("No name processor provided")
	}

	host := &host{nameProcessor: np}
	if err := host.SetName(name); err != nil {
		return nil, err
	}

	return host, nil
}

// GetNameProcessor returns the name.Processor that is associated with the host.
func (h *host) GetNameProcessor() name.Processor {
	return h.nameProcessor
}

// SetNameProcessor allows for the name.Processor that is associated with the host to be overridden.
func (h *host) SetNameProcessor(np name.Processor) error {
	h.nameProcessor = np
	return h.SetName(h.providedName)
}

// GetName returns the name of the host.
func (h *host) GetName() string {
	return h.workingName
}

// SetName allows for the current host name to be overridden.
func (h *host) SetName(name string) error {
	if !h.nameProcessor.IsValid(name) {
		return errors.New(fmt.Sprintf("Invalid hostname provided: '%s'", name))
	}

	providedName := name

	if h.nameProcessor != nil {
		var err error
		name, err = h.nameProcessor.FormatName(name)
		if err != nil {
			return errors.Wrap(err, "Failed to set name")
		}
	}

	// record the provided name so that we can reformat it later if the name.Processor changes
	h.providedName = providedName
	h.workingName = name

	return nil
}
