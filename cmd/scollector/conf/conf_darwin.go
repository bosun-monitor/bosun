package conf

// ProcessParams is not used on Mac OS
type ProcessParams struct{}

// ServiceParams holds service parameters
type ServiceParams struct {
	Name      string
	WatchProc bool
}

// InitializeSWbemServices is an initializer for SWbemServices
func (c *Conf) InitializeSWbemServices() {}
