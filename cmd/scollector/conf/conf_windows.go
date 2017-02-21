package conf

import (
	"bosun.org/slog"
	"github.com/StackExchange/wmi"
)

type ProcessParams struct {
	Name string
}

type ServiceParams struct {
	Name      string
	WatchProc bool
}

func (c *Conf) InitializeSWbemServices() {
	slog.Infof("Initializing SWbemServices")
	s, err := wmi.InitializeSWbemServices(wmi.DefaultClient)
	if err != nil {
		slog.Fatal(err)
	}
	wmi.DefaultClient.SWbemServicesClient = s
}
