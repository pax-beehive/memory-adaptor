package cli

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/pax-beehive/memory-adaptor/internal/adapters"
	"github.com/pax-beehive/memory-adaptor/internal/capturequeue"
	"github.com/pax-beehive/memory-adaptor/internal/config"
	"github.com/pax-beehive/memory-adaptor/internal/facade"
)

func TestHookDaemonLockAllowsOnlyOneOwnerAndRecoversAfterRelease(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	release, err := acquireHookDaemonLock(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acquireHookDaemonLock(configPath); err == nil {
		t.Fatal("second daemon unexpectedly acquired the same lock")
	}
	release()
	releaseAgain, err := acquireHookDaemonLock(configPath)
	if err != nil {
		t.Fatalf("lock was not reusable after release: %v", err)
	}
	releaseAgain()
}

func TestCaptureSessionKeyIncludesTargetWorkspaceAndSession(t *testing.T) {
	first := captureSessionKey(facade.HookEvent{Target: "codex", Workspace: "/workspace/a", Metadata: map[string]string{"session_id": "same"}})
	second := captureSessionKey(facade.HookEvent{Target: "codex", Workspace: "/workspace/b", Metadata: map[string]string{"session_id": "same"}})
	third := captureSessionKey(facade.HookEvent{Target: "claude", Workspace: "/workspace/a", Metadata: map[string]string{"session_id": "same"}})
	if first == second || first == third || second == third {
		t.Fatalf("capture partitions collided: %q %q %q", first, second, third)
	}
	unknownA := captureSessionKey(facade.HookEvent{Target: "codex", Workspace: "/workspace/a", Metadata: map[string]string{"event_id": "a"}})
	unknownB := captureSessionKey(facade.HookEvent{Target: "codex", Workspace: "/workspace/a", Metadata: map[string]string{"event_id": "b"}})
	if unknownA == unknownB {
		t.Fatalf("unidentified sessions were collapsed: %q", unknownA)
	}
}

func TestCaptureQueueHookAcknowledgesDurableTerminalWithoutWaitingForProvider(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := config.DefaultConfig(configPath)
	provider := cfg.Providers["sqlite"]
	provider.Path = filepath.Join(t.TempDir(), "memory.sqlite")
	cfg.Providers["sqlite"] = provider
	router, err := adapters.DefaultRegistry().BuildRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	service := facade.New(cfg, router)
	providerStarted := make(chan struct{})
	releaseProvider := make(chan struct{})
	queue, err := capturequeue.Open(filepath.Join(t.TempDir(), "capture.sqlite"), capturequeue.Options{
		Providers: func(string) []string { return []string{"sqlite"} },
		Deliver: func(context.Context, string, capturequeue.Episode) (string, error) {
			close(providerStarted)
			<-releaseProvider
			return "ref", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer queue.Close()
	worker := newCaptureDeliveryWorker(queue)
	defer worker.Close()
	server, client := net.Pipe()
	done := make(chan error, 1)
	go func() {
		_, _, err := handleCaptureQueueConn(context.Background(), service, queue, server, worker.Notify, func() {})
		done <- err
	}()
	raw := json.RawMessage(`{"session_id":"session-a","last_assistant_message":"done"}`)
	if err := json.NewEncoder(client).Encode(hookBufferRequest{Target: "codex", Event: "turn_end", Raw: raw}); err != nil {
		t.Fatal(err)
	}
	var response hookBufferResponse
	if err := json.NewDecoder(client).Decode(&response); err != nil {
		t.Fatal(err)
	}
	client.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !response.OK || !response.Buffered || response.Flushed != 1 {
		t.Fatalf("unexpected response: %#v", response)
	}
	select {
	case <-providerStarted:
	case <-time.After(time.Second):
		t.Fatal("delivery worker did not start")
	}
	stats, err := queue.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.PendingDeliveries != 1 {
		t.Fatalf("terminal was not durably queued: %#v", stats)
	}
	close(releaseProvider)
}
