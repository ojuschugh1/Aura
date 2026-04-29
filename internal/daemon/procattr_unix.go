//go:build !windows

package daemon

import "syscall"

// daemonSysProcAttr returns SysProcAttr that creates a new session (setsid)
// so the daemon is fully detached from the terminal on Unix systems.
func daemonSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
