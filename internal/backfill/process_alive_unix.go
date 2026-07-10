//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package backfill

import (
	"errors"
	"syscall"
)

func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
