package adapters

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pax-beehive/memory-adaptor/internal/config"
)

func TestBuildRouterUsesProfileRequiredForHealth(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig(filepath.Join(t.TempDir(), "config.yaml"))
	recall := cfg.RecallProfiles["default"]
	recall.Providers[0].Required = false
	cfg.RecallProfiles["default"] = recall
	write := cfg.WriteProfiles["default"]
	write.Providers[0].Required = false
	cfg.WriteProfiles["default"] = write

	router, err := DefaultRegistry().BuildRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	statuses, err := router.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 1 || statuses[0].Required {
		t.Fatalf("expected best-effort health status, got %#v", statuses)
	}
}
