package conf

type ProcessParams struct{}

type ServiceParams struct {
	Name      string
	WatchProc bool
}

func (c *Conf) InitializeSWbemServices() {}
