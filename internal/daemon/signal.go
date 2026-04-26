package daemon

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
)

// HandleSignals sets up SIGTERM/SIGINT handling and a panic recovery for the daemon process.
// onShutdown is called once before the process exits.
func HandleSignals(dir string, onShutdown func()) {
	// Recover from unexpected panics and write a crash log.
	defer func() {
		if r := recover(); r != nil {
			writeCrashLog(dir, fmt.Sprintf("panic: %v\n%s", r, debug.Stack()))
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	<-ch
	signal.Stop(ch)

	if onShutdown != nil {
		onShutdown()
	}
}

// RecoverAndLog wraps a function call with panic recovery, writing a crash log on panic.
func RecoverAndLog(dir string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("panic: %v\n%s", r, debug.Stack())
			writeCrashLog(dir, msg)
			err = fmt.Errorf("%s", msg)
		}
	}()
	return fn()
}

// writeCrashLog writes msg to .aura/crash.log.
func writeCrashLog(dir, msg string) {
	path := filepath.Join(dir, "crash.log")
	_ = os.WriteFile(path, []byte(msg), 0644)
}
