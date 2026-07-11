package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pax-beehive/memory-adaptor/internal/capturequeue"
	"github.com/pax-beehive/memory-adaptor/internal/config"
)

type captureDeliveryWorker struct {
	queue  *capturequeue.Queue
	notify chan struct{}
	stop   chan struct{}
	done   chan struct{}
}

func newCaptureDeliveryWorker(queue *capturequeue.Queue) *captureDeliveryWorker {
	worker := &captureDeliveryWorker{
		queue:  queue,
		notify: make(chan struct{}, 1),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	go func() {
		defer close(worker.done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-worker.notify:
				_, _ = worker.queue.RunOnce(context.Background())
			case <-ticker.C:
				_, _ = worker.queue.SealExpired(context.Background())
				_, _ = worker.queue.RunOnce(context.Background())
			case <-worker.stop:
				return
			}
		}
	}()
	return worker
}

func (w *captureDeliveryWorker) Notify() {
	select {
	case w.notify <- struct{}{}:
	default:
	}
}

func (w *captureDeliveryWorker) Close() {
	close(w.stop)
	<-w.done
}

func hookQueuePath(configPath string) string {
	return filepath.Join(filepath.Dir(config.ExpandPath(configPath)), "hooks", "capture.sqlite")
}

func acquireHookDaemonLock(configPath string) (func(), error) {
	lockPath := filepath.Join(filepath.Dir(config.ExpandPath(configPath)), "hooks", "paxm-hook.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		file, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			if _, err := fmt.Fprintf(file, "%d\n", os.Getpid()); err != nil {
				file.Close()
				os.Remove(lockPath)
				return nil, err
			}
			return func() {
				file.Close()
				_ = os.Remove(lockPath)
			}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		contents, readErr := os.ReadFile(lockPath)
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(contents)))
		if readErr == nil && parseErr == nil && pid > 0 {
			if process, findErr := os.FindProcess(pid); findErr == nil && process.Signal(syscall.Signal(0)) == nil {
				return nil, fmt.Errorf("paxm hook daemon already running with pid %d", pid)
			}
		}
		if err := os.Remove(lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return nil, errors.New("could not acquire paxm hook daemon lock")
}
