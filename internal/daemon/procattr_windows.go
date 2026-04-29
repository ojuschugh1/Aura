//go:build windows

package daemon

import "syscall"

// daemonSysProcAttr returns SysProcAttr for Windows.
// Windows doesn't have setsid; the CREATE_NEW_PROCESS_GROUP flag
// detaches the child from the parent's console.
func daemonSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: 0x00000200, // CREATE_NEW_PROCESS_GROUP
	}
}
