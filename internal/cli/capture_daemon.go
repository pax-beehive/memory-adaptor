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
	cancel context.CancelFunc
}

func newCaptureDeliveryWorker(queue *capturequeue.Queue) *captureDeliveryWorker {
	ctx, cancel := context.WithCancel(context.Background())
	worker := &captureDeliveryWorker{
		queue:  queue,
		notify: make(chan struct{}, 1),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
		cancel: cancel,
	}
	go func() {
		defer close(worker.done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-worker.notify:
				_, _ = worker.queue.RunOnce(ctx)
			case <-ticker.C:
				_, _ = worker.queue.SealExpired(ctx)
				_, _ = worker.queue.RunOnce(ctx)
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
	w.cancel()
	close(w.stop)
	select {
	case <-w.done:
	case <-time.After(time.Second):
	}
}

func hookQueuePath(configPath string) string {
	return filepath.Join(filepath.Dir(config.ExpandPath(configPath)), "hooks", "capture.sqlite")
}

func acquireHookDaemonLock(configPath string) (func(), error) {
	lockPath := hookDaemonLockPath(configPath)
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

func hookDaemonLockPath(configPath string) string {
	return filepath.Join(filepath.Dir(config.ExpandPath(configPath)), "hooks", "paxm-hook.lock")
}
