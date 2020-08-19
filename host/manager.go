package host

import (
	"os"

	"bosun.org/name"
	"github.com/pkg/errors"
)

type manager struct {
	nameProcessor name.Processor
	host          Host
}

// Manager is an interface for types which manage hosts
//
// GetNameProcessor returns a name.Processor which is suitable for hosts
//
// GetHost returns the Host that represents the machine which the process is running on
//
// GetHostName returns the name of the managed host - is simply more convenient than calling manager.GetHost().GetName()
type Manager interface {
	GetNameProcessor() name.Processor
	GetHost() Host
	GetHostName() string
}

var hostname = os.Hostname

// NewManager constructs a new Manager for a host which is named by the operating system
func NewManager(preserveFullHostName bool) (Manager, error) {
	n, err := hostname()
	// Prob better to return an error but this is established behavior not worth introducing a breaking change for
	if err != nil {
		n = "unknown"
	}

	return NewManagerForHostname(n, preserveFullHostName)
}

// NewManagerForHostname constructs a new Manager for a host which is named according to the 'hostname' parameter
func NewManagerForHostname(hostname string, preserveFullHostName bool) (Manager, error) {
	if hostname == "" {
		return nil, errors.New("No 'hostname' provided")
	}

	processor, err := NewHostNameProcessor(preserveFullHostName)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create host name processor")
	}

	host, err := NewHost(hostname, processor)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to construct host")
	}

	f := &manager{nameProcessor: processor, host: host}

	return f, nil
}

// GetNameProcessor returns a name.Processor which is suitable for hosts
func (m *manager) GetNameProcessor() name.Processor {
	return m.nameProcessor
}

// GetHost returns the Host that represents the machine which the process is running on
func (m *manager) GetHost() Host {
	return m.host
}

// GetHostName returns the name of the managed host - is simply more convenient than calling m.GetHost().GetName()
func (m *manager) GetHostName() string {
	return m.host.GetName()
}
