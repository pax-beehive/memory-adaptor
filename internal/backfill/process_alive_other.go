//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package backfill

func processAlive(_ int) bool {
	return false
}
