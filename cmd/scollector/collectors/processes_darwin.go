package collectors

import "fmt"

func WatchProcesses(procs []*WatchedProc) error {
	return fmt.Errorf("process watching not implemented on Darwin")
}
