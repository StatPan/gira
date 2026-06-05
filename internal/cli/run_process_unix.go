//go:build !windows

package cli

import (
	"os/exec"
	"syscall"
)

func configureRunProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
