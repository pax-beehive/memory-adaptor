package backfill

import "os"

func processID() int {
	return os.Getpid()
}
