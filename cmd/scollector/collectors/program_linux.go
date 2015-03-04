package collectors

import (
	"os/exec"
	"syscall"
)

func init() {
	setupExternalCommand = func(cmd *exec.Cmd) {
		cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	}
}
