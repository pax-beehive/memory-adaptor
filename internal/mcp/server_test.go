package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pax-beehive/paxm/internal/config"
	"github.com/pax-beehive/paxm/internal/facade"
	"github.com/pax-beehive/paxm/internal/memory"
)

func TestServerServesMemoryToolsOverStdio(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := config.DefaultConfig(configPath)
	cfg.Identity.UserID = "todd"
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"dev"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"paxm_remember","arguments":{"text":"paxm mcp mode remembers provider fan-out","metadata":{"topic":"mcp"}}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"paxm_recall","arguments":{"query":"mcp provider fan-out","limit":3}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"paxm_history","arguments":{"days":7}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"paxm_config_doctor","arguments":{}}}`,
	}, "\n") + "\n"

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := Serve(Options{
		ConfigPath: configPath,
		AgentName:  "codex",
		Version:    "test",
		Stdin:      strings.NewReader(input),
		Stdout:     &stdout,
		Stderr:     &stderr,
	}); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}

	responses := decodeResponses(t, stdout.String())
	if len(responses) != 6 {
		t.Fatalf("expected 4 responses, got %d: %s", len(responses), stdout.String())
	}
	assertNoRPCError(t, responses)

	var initResult map[string]any
	decodeResult(t, responses[0], &initResult)
	if initResult["protocolVersion"] != protocolVersion {
		t.Fatalf("unexpected initialize result: %#v", initResult)
	}

	var listResult struct {
		Tools []toolDefinition `json:"tools"`
	}
	decodeResult(t, responses[1], &listResult)
	if got := toolNames(listResult.Tools); strings.Join(got, ",") != "paxm_recall,paxm_remember,paxm_history,paxm_config_doctor" {
		t.Fatalf("unexpected tools: %#v", got)
	}

	var rememberResult toolResult
	decodeResult(t, responses[2], &rememberResult)
	if rememberResult.IsError || !strings.Contains(rememberResult.Content[0].Text, `"refs"`) {
		t.Fatalf("unexpected remember result: %#v", rememberResult)
	}

	var recallResult toolResult
	decodeResult(t, responses[3], &recallResult)
	if recallResult.IsError || !strings.Contains(recallResult.Content[0].Text, "paxm mcp mode remembers") {
		t.Fatalf("unexpected recall result: %#v", recallResult)
	}
	for _, marker := range []string{`<paxm-recall version="1" mode="active">`, `</paxm-recall>`} {
		if !strings.Contains(recallResult.Content[0].Text, marker) {
			t.Fatalf("recall result omitted envelope %q: %#v", marker, recallResult)
		}
	}
	for _, provenance := range []string{`"scope_type": "personal"`, `"scope_id": "todd"`, `"user_id": "todd"`, `"agent_id": "codex-todd"`} {
		if !strings.Contains(recallResult.Content[0].Text, provenance) {
			t.Fatalf("recall result omitted %q: %#v", provenance, recallResult)
		}
	}
	structured, ok := recallResult.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("recall structured content has unexpected type: %#v", recallResult.StructuredContent)
	}
	context, ok := structured["paxm_context"].(map[string]any)
	if !ok || context["kind"] != "recall" || context["mode"] != "active" {
		t.Fatalf("recall structured content omitted provenance: %#v", structured)
	}

	var historyResult toolResult
	decodeResult(t, responses[4], &historyResult)
	if historyResult.IsError || !strings.Contains(historyResult.Content[0].Text, `"recalls"`) || !strings.Contains(historyResult.Content[0].Text, `"writes"`) {
		t.Fatalf("unexpected history result: %#v", historyResult)
	}

	var doctorResult toolResult
	decodeResult(t, responses[5], &doctorResult)
	if doctorResult.IsError || !strings.Contains(doctorResult.Content[0].Text, `"provider": "sqlite"`) {
		t.Fatalf("unexpected doctor result: %#v", doctorResult)
	}

}

func TestServerActiveToolsShareRuntimeSessionIdentity(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	sessionPath := filepath.Join(t.TempDir(), "session-id")
	cfg := config.DefaultConfig(configPath)
	cfg.Providers["sqlite"] = config.ProviderConfig{Type: "sqlite", Enabled: false}
	cfg.Providers["team"] = config.ProviderConfig{
		Type:      "jsonrpc",
		Enabled:   true,
		Transport: "stdio",
		Command:   os.Args[0],
		Args:      []string{"-test.run=TestMCPActiveSessionProviderHelper", "--"},
		Env: map[string]string{
			"PAXM_MCP_ACTIVE_SESSION_HELPER": "1",
			"PAXM_MCP_ACTIVE_SESSION_FILE":   sessionPath,
		},
		Timeout: "5s",
	}
	recall := cfg.RecallProfiles["default"]
	recall.Providers = []config.ProviderRouteConfig{{Name: "team", Required: true}}
	cfg.RecallProfiles["default"] = recall
	write := cfg.WriteProfiles["default"]
	write.Providers = []config.ProviderRouteConfig{{Name: "team", Required: true}}
	cfg.WriteProfiles["default"] = write
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"paxm_remember","arguments":{"text":"stable MCP session","metadata":{"session_id":"caller-spoof"}}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"paxm_recall","arguments":{"query":"stable MCP session","meta":{"session_id":"caller-spoof"}}}}`,
	}, "\n") + "\n"
	var stdout bytes.Buffer
	if err := Serve(Options{ConfigPath: configPath, AgentName: "codex", Stdin: strings.NewReader(input), Stdout: &stdout}); err != nil {
		t.Fatal(err)
	}
	responses := decodeResponses(t, stdout.String())
	if len(responses) != 2 {
		t.Fatalf("responses = %d, want 2: %s", len(responses), stdout.String())
	}
	assertNoRPCError(t, responses)
	var remembered toolResult
	decodeResult(t, responses[0], &remembered)
	if remembered.IsError || !strings.Contains(remembered.Content[0].Text, `"provider": "team"`) || strings.Contains(remembered.Content[0].Text, "provider_errors") {
		t.Fatalf("unexpected remember result: %#v", remembered)
	}
	var recalled toolResult
	decodeResult(t, responses[1], &recalled)
	if recalled.IsError || !strings.Contains(recalled.Content[0].Text, "stable MCP session") ||
		!strings.Contains(recalled.Content[0].Text, `"outcome": "success"`) ||
		strings.Contains(recalled.Content[0].Text, "provider_errors") {
		t.Fatalf("unexpected recall result: %#v", recalled)
	}
}

func TestMCPActiveSessionProviderHelper(t *testing.T) {
	if os.Getenv("PAXM_MCP_ACTIVE_SESSION_HELPER") != "1" {
		return
	}
	var request struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		t.Fatal(err)
	}
	sessionPath := os.Getenv("PAXM_MCP_ACTIVE_SESSION_FILE")
	var result any
	switch request.Method {
	case "paxm.putBatch":
		var batch struct {
			Items []memory.MemoryItem `json:"items"`
		}
		if err := json.Unmarshal(request.Params, &batch); err != nil {
			t.Fatal(err)
		}
		if len(batch.Items) != 1 {
			t.Fatalf("putBatch items = %d, want 1", len(batch.Items))
		}
		sessionID := strings.TrimSpace(batch.Items[0].Origin.SessionID)
		if sessionID == "" {
			t.Fatal("put origin.session_id is required")
		}
		if sessionID == "caller-spoof" {
			t.Fatal("caller metadata replaced trusted MCP runtime session")
		}
		if batch.Items[0].Metadata["session_id"] != "" {
			t.Fatalf("caller session_id leaked as raw provider metadata: %#v", batch.Items[0].Metadata)
		}
		if err := os.WriteFile(sessionPath, []byte(sessionID), 0o600); err != nil {
			t.Fatal(err)
		}
		result = map[string]any{"refs": []map[string]string{{"id": "team-memory-1"}}}
	case "paxm.search":
		var query memory.SearchQuery
		if err := json.Unmarshal(request.Params, &query); err != nil {
			t.Fatal(err)
		}
		sessionID := strings.TrimSpace(query.Metadata["session_id"])
		if sessionID == "" {
			t.Fatal("search metadata.session_id is required")
		}
		if sessionID == "caller-spoof" {
			t.Fatal("caller metadata replaced trusted MCP runtime session")
		}
		written, err := os.ReadFile(sessionPath)
		if err != nil {
			t.Fatal(err)
		}
		if sessionID != string(written) {
			t.Fatalf("search session_id = %q, want stable %q", sessionID, written)
		}
		result = map[string]any{"hits": []memory.MemoryHit{{
			ID: "team-memory-1", Text: "stable MCP session", Relevance: 1, Score: 1,
		}}}
	default:
		t.Fatalf("unexpected method %q", request.Method)
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      request.ID,
		"result":  result,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestServerLabelsMissingScopeUnknown(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := config.Save(configPath, config.DefaultConfig(configPath)); err != nil {
		t.Fatal(err)
	}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"paxm_remember","arguments":{"text":"legacy scope marker"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"paxm_recall","arguments":{"query":"legacy scope marker"}}}`,
	}, "\n") + "\n"
	var stdout bytes.Buffer
	if err := Serve(Options{ConfigPath: configPath, Stdin: strings.NewReader(input), Stdout: &stdout}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"scope_type":"unknown"`) {
		t.Fatalf("structured recall omitted unknown scope: %s", stdout.String())
	}
}

func TestRecallErrorToolResultMarksPartialHits(t *testing.T) {
	t.Parallel()
	result := recallErrorToolResult(errors.New("required provider failed"), facade.RecallResult{
		Query: "deployment",
		Hits:  []memory.MemoryHit{{Provider: "sqlite", ID: "partial", Text: "partial recalled memory"}},
	})
	if !result.IsError || !strings.Contains(result.Content[0].Text, `<paxm-recall version="1" mode="active">`) || !strings.Contains(result.Content[0].Text, "partial recalled memory") {
		t.Fatalf("partial recall error omitted text provenance: %#v", result)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("partial recall structured content has unexpected type: %#v", result.StructuredContent)
	}
	context, ok := structured["paxm_context"].(map[string]any)
	if !ok || context["kind"] != "recall" || context["mode"] != "active" {
		t.Fatalf("partial recall error omitted structured provenance: %#v", structured)
	}
}

func TestServerCachesAndReloadsRuntime(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := config.Save(configPath, config.DefaultConfig(configPath)); err != nil {
		t.Fatal(err)
	}
	server := NewServer(Options{ConfigPath: configPath})

	first, err := server.runtime()
	if err != nil {
		t.Fatal(err)
	}
	second, err := server.runtime()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("expected runtime to be cached across calls")
	}

	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(configPath, future, future); err != nil {
		t.Fatal(err)
	}
	reloaded, err := server.runtime()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded == first {
		t.Fatal("expected runtime reload after config change")
	}

	server.closeRuntime()
	if server.rt != nil {
		t.Fatal("expected closeRuntime to release the cached runtime")
	}
}

func TestServerClosesRuntimeAfterServe(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := config.Save(configPath, config.DefaultConfig(configPath)); err != nil {
		t.Fatal(err)
	}
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"paxm_recall","arguments":{"query":"anything"}}}` + "\n"
	var stdout bytes.Buffer
	server := NewServer(Options{
		ConfigPath: configPath,
		Stdin:      strings.NewReader(input),
		Stdout:     &stdout,
	})
	if err := server.Serve(context.Background()); err != nil {
		t.Fatal(err)
	}
	if server.rt != nil {
		t.Fatal("expected cached runtime to be closed after Serve returned")
	}
}

func TestServerParseErrorRespondsWithNullID(t *testing.T) {
	var stdout bytes.Buffer
	if err := Serve(Options{
		Stdin:  strings.NewReader("{not-json}\n"),
		Stdout: &stdout,
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"id":null`) || !strings.Contains(stdout.String(), `"code":-32700`) {
		t.Fatalf("unexpected parse error response: %s", stdout.String())
	}
}

func TestServerRejectsInvalidToolArguments(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := config.DefaultConfig(configPath)
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"paxm_recall","arguments":{"query":"x","extra":true}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"paxm_recall","arguments":{"query":"x","limit":0}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"paxm_recall","arguments":{"query":"   "}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"paxm_history","arguments":{"days":0}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"paxm_remember","arguments":{"text":""}}}`,
	}, "\n") + "\n"

	var stdout bytes.Buffer
	if err := Serve(Options{
		ConfigPath: configPath,
		Stdin:      strings.NewReader(input),
		Stdout:     &stdout,
	}); err != nil {
		t.Fatal(err)
	}
	responses := decodeResponses(t, stdout.String())
	assertNoRPCError(t, responses)
	if len(responses) != 5 {
		t.Fatalf("expected 4 responses, got %d: %s", len(responses), stdout.String())
	}
	for _, response := range responses {
		var result toolResult
		decodeResult(t, response, &result)
		if !result.IsError {
			t.Fatalf("expected tool error for id %s: %#v", response.ID, result)
		}
	}
	output := stdout.String()
	for _, expected := range []string{
		"unknown field",
		"limit must be at least 1",
		"query is required",
		"days must be at least 1",
		"text is required",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("missing %q in invalid argument output: %s", expected, output)
		}
	}
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func decodeResponses(t *testing.T, output string) []rpcResponse {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	responses := make([]rpcResponse, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var response rpcResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			t.Fatalf("invalid response %q: %v", line, err)
		}
		responses = append(responses, response)
	}
	return responses
}

func assertNoRPCError(t *testing.T, responses []rpcResponse) {
	t.Helper()
	for _, response := range responses {
		if response.Error != nil {
			t.Fatalf("unexpected rpc error for id %s: %#v", response.ID, response.Error)
		}
	}
}

func decodeResult(t *testing.T, response rpcResponse, out any) {
	t.Helper()
	if err := json.Unmarshal(response.Result, out); err != nil {
		t.Fatalf("decode result for id %s: %v\n%s", response.ID, err, string(response.Result))
	}
}

func toolNames(tools []toolDefinition) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}
