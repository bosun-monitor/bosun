package collectors

import (
	"bosun.org/cmd/scollector/conf"
	"fmt"
	"runtime"
)

func AddProcessConfig(params conf.ProcessParams) error {
	return fmt.Errorf("process monitoring not supported on %s-%s", runtime.GOOS, runtime.GOARCH)
}

func WatchProcesses() {}
