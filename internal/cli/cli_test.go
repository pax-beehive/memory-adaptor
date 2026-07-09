package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLISetupRememberRecallAndHook(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Main([]string{"--config", configPath, "setup"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("setup failed with code %d: %s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Main([]string{"--config", configPath, "remember", "--text", "paxm uses hook passive recall and provider fan-out"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("remember failed with code %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "stored memory") {
		t.Fatalf("unexpected remember output: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Main([]string{"--config", configPath, "recall", "--query", "passive recall"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("recall failed with code %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "hook passive recall") {
		t.Fatalf("unexpected recall output: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	event := strings.NewReader(`{"prompt":"passive recall","workspace":"/tmp/project"}`)
	code = Main([]string{"--config", configPath, "hook", "run", "--json"}, event, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("hook run failed with code %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"recall"`) || !strings.Contains(stdout.String(), "provider fan-out") {
		t.Fatalf("unexpected hook output: %s", stdout.String())
	}
}

func TestCLIProviderEnablePreservesRequiredByDefault(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if code := Main([]string{"--config", configPath, "setup"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("setup failed with code %d: %s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := Main([]string{"--config", configPath, "provider", "disable", "local"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("disable failed with code %d: %s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := Main([]string{"--config", configPath, "provider", "enable", "local", "--read=false", "--write=true"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("enable failed with code %d: %s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := Main([]string{"--config", configPath, "provider", "list"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("list failed with code %d: %s", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "local\tlocal\ttrue\tfalse\ttrue\ttrue") {
		t.Fatalf("provider flags were not preserved: %s", output)
	}
}
