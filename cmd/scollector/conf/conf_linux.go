package conf

type ProcessParams struct {
	Command      string
	Name         string
	Args         string
	IncludeCount bool
}

type ServiceParams struct {
	Name      string
	WatchProc bool
}

func (c *Conf) InitializeSWbemServices() {}
