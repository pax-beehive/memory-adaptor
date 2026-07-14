package cli

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/pax-beehive/paxm/internal/config"
)

func TestPromptOpenVikingProvider(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig(t.TempDir() + "/config.yaml")
	var output bytes.Buffer
	input := strings.NewReader("\nsecret\n1\n2\n")
	if err := promptOpenVikingProvider(bufio.NewReader(input), &output, &cfg, "openviking"); err != nil {
		t.Fatal(err)
	}
	provider := cfg.Providers["openviking"]
	if provider.BaseURL != config.DefaultOpenVikingBaseURL() || provider.APIKey != "secret" {
		t.Fatalf("provider = %#v", provider)
	}
	if !recallProfileHasProvider(cfg.RecallProfiles["default"], "openviking") || !writeProfileHasProvider(cfg.WriteProfiles["default"], "openviking") {
		t.Fatalf("OpenViking was not routed for read/write")
	}
	if required, _ := config.ProviderRouteRequired(cfg.RecallProfiles["default"].Providers, "openviking"); required {
		t.Fatalf("OpenViking should be best effort")
	}
}
