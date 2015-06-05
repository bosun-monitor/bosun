package collectors

import (
	"fmt"
	"runtime"
)

func AddProcessConfig(params ProcessParams) error {
	return fmt.Errorf("process monitoring not supported on %s-%s", runtime.GOOS, runtime.GOARCH)
}

type ProcessParams struct{}

func WatchProcesses() {}
