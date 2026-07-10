package backfill

import (
	"errors"
	"testing"
	"time"
)

func TestStorePreventsConcurrentRunsAndRemembersSucceededItems(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	scope := Scope("/tmp/config.yaml", "codex", "mem0-company")

	release, err := store.Acquire(scope, "run-one")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Acquire(scope, "run-two"); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second acquire error = %v, want ErrAlreadyRunning", err)
	}
	if err := store.MarkSucceeded(scope, "turn-one", "provider-ref"); err != nil {
		t.Fatal(err)
	}
	done, err := store.Succeeded(scope, "turn-one")
	if err != nil || !done {
		t.Fatalf("succeeded = %v, err = %v", done, err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	secondRelease, err := store.Acquire(scope, "run-two")
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	_ = secondRelease()
}

func TestReadStatusMarksDeadBackgroundWorkerInterrupted(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	scope := Scope("config", "codex", "target")
	if err := store.WriteStatus(scope, Status{State: "running", PID: 99999999, Agent: "codex", Provider: "target", StartedAt: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	status, err := store.ReadStatus(scope)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "interrupted" || status.FinishedAt.IsZero() {
		t.Fatalf("dead worker status was not reconciled: %#v", status)
	}
}
