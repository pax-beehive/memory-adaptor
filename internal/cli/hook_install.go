package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pax-beehive/paxm/internal/config"
)

func removeLegacyHookShim(configPath, target string) error {
	legacyPath := filepath.Join(filepath.Dir(config.ExpandPath(configPath)), "hooks", target+"-user_prompt")
	if err := os.Remove(legacyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

type hookInstallEvent struct {
	ConfigEvent string
	NativeEvent string
	Matcher     string
	Status      string
}

func installedHookEvents() []hookInstallEvent {
	return []hookInstallEvent{
		{
			ConfigEvent: "session_start",
			NativeEvent: "SessionStart",
			Matcher:     "startup|resume|clear|compact",
			Status:      "Buffering paxm session memory",
		},
		{
			ConfigEvent: "user_input",
			NativeEvent: "UserPromptSubmit",
			Status:      "Recalling paxm memory",
		},
		{
			ConfigEvent: "tool_use",
			NativeEvent: "PostToolUse",
			Status:      "Buffering paxm tool memory",
		},
		{
			ConfigEvent: "tool_failure",
			NativeEvent: "PostToolUseFailure",
			Status:      "Buffering failed paxm tool memory",
		},
		{
			ConfigEvent: "turn_end",
			NativeEvent: "Stop",
			Status:      "Buffering paxm turn memory",
		},
	}
}

func hookInstallEventByConfig(configEvent string) (hookInstallEvent, bool) {
	for _, event := range installedHookEvents() {
		if event.ConfigEvent == configEvent {
			return event, true
		}
	}
	return hookInstallEvent{}, false
}

func installHookShim(configPath, target, event string) (string, error) {
	hooksDir := filepath.Join(filepath.Dir(config.ExpandPath(configPath)), "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return "", err
	}
	installEvent, ok := hookInstallEventByConfig(event)
	if !ok {
		return "", fmt.Errorf("unsupported hook event %q", event)
	}
	binaryPath, err := os.Executable()
	if err != nil || binaryPath == "" {
		binaryPath = "paxm"
	}
	scriptPath := filepath.Join(hooksDir, target+"-"+event)
	outputFlag := " --json"
	switch target {
	case "claude", "trae", "trae-cn", "kiro":
		outputFlag = ""
	case "kimi":
		outputFlag = " --kimi"
	case "cline":
		outputFlag = " --cline"
	case "cursor":
		outputFlag = " --cursor"
	case "zcode":
		outputFlag = " --zcode"
	}
	extension, script := hookShimScript(runtime.GOOS, binaryPath, config.ExpandPath(configPath), target, event, outputFlag)
	scriptPath += extension
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		return "", err
	}
	if target == "codex" {
		if err := installCodexGlobalHook(codexConfigPath(), scriptPath, installEvent.ConfigEvent); err != nil {
			return "", err
		}
	}
	return scriptPath, nil
}

func hookShimScript(goos, binaryPath, configPath, target, event, outputFlag string) (string, string) {
	if goos == "windows" {
		script := "& " + powerShellQuote(binaryPath) + " --config " + powerShellQuote(configPath) + " __hook --target " + powerShellQuote(target) + " --event " + powerShellQuote(event) + outputFlag + "\nexit 0\n"
		return ".ps1", script
	}
	script := "#!/bin/sh\n" + shellQuote(binaryPath) + " --config " + shellQuote(configPath) + " __hook --target " + shellQuote(target) + " --event " + shellQuote(event) + outputFlag + " || exit 0\n"
	return "", script
}

func codexConfigPath() string {
	if path := os.Getenv("PAXM_CODEX_CONFIG"); path != "" {
		return config.ExpandPath(path)
	}
	codexHome := os.Getenv("CODEX_HOME")
	if codexHome == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return filepath.Join(".codex", "config.toml")
		}
		codexHome = filepath.Join(home, ".codex")
	}
	return filepath.Join(config.ExpandPath(codexHome), "config.toml")
}

func claudeSettingsPath() string {
	if path := os.Getenv("PAXM_CLAUDE_SETTINGS"); path != "" {
		return config.ExpandPath(path)
	}
	claudeConfigDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if claudeConfigDir == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return filepath.Join(".claude", "settings.json")
		}
		claudeConfigDir = filepath.Join(home, ".claude")
	}
	return filepath.Join(config.ExpandPath(claudeConfigDir), "settings.json")
}

func piAgentDir() string {
	if path := os.Getenv("PAXM_PI_AGENT_DIR"); path != "" {
		return config.ExpandPath(path)
	}
	if path := os.Getenv("PI_CODING_AGENT_DIR"); path != "" {
		return config.ExpandPath(path)
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".pi", "agent")
	}
	return filepath.Join(home, ".pi", "agent")
}

func piExtensionPath() string {
	return filepath.Join(piAgentDir(), "extensions", "paxm-hook", "index.ts")
}

func installPiGlobalHook(path string, scriptPaths map[string]string) error {
	sessionStartScriptPath := strings.TrimSpace(scriptPaths["session_start"])
	userInputScriptPath := strings.TrimSpace(scriptPaths["user_input"])
	turnEndScriptPath := strings.TrimSpace(scriptPaths["turn_end"])
	if sessionStartScriptPath == "" && userInputScriptPath == "" && turnEndScriptPath == "" {
		return errors.New("pi hook requires at least one hook shim")
	}
	path = config.ExpandPath(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(piHookExtensionSource(sessionStartScriptPath, userInputScriptPath, turnEndScriptPath)), 0o644)
}

func piHookExtensionSource(sessionStartScriptPath, userInputScriptPath, turnEndScriptPath string) string {
	sessionStartScriptLiteral := jsonStringLiteral(config.ExpandPath(sessionStartScriptPath))
	userInputScriptLiteral := jsonStringLiteral(config.ExpandPath(userInputScriptPath))
	turnEndScriptLiteral := jsonStringLiteral(config.ExpandPath(turnEndScriptPath))
	return `import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { spawnSync } from "node:child_process";

const paxmSessionStartHookCommand = ` + sessionStartScriptLiteral + `;
const paxmUserInputHookCommand = ` + userInputScriptLiteral + `;
const paxmTurnEndHookCommand = ` + turnEndScriptLiteral + `;
const maxBufferedMessages = 20;

type BufferedMessage = {
  role: string;
  text: string;
  source: string;
};

let activeContext: any;
let lastPrompt = "";
let pendingSessionContext = "";
let turnMessages: BufferedMessage[] = [];
const pendingToolArgs = new Map<string, any>();

function currentSessionId(ctx: any): string {
  const sessionFile = ctx.sessionManager?.getSessionFile?.();
  if (typeof sessionFile !== "string") return "";
  const fileName = sessionFile.split(/[\\/]/).pop() ?? "";
  const timestamped = fileName.match(/^\d{4}-\d{2}-\d{2}T[^_]+_(.+)\.jsonl$/i);
  if (timestamped?.[1]) return timestamped[1];
  return fileName.replace(/\.jsonl$/i, "");
}

function currentWorkspace(ctx: any): string {
  if (typeof ctx?.cwd === "string") return ctx.cwd;
  return "";
}

function activeCtx(ctx: any): any {
  if (ctx) activeContext = ctx;
  return ctx ?? activeContext ?? {};
}

function sanitizeValue(value: any): any {
  if (Array.isArray(value)) return value.map(sanitizeValue).filter((item) => item !== undefined);
  if (value && typeof value === "object") {
    const kind = String(value.type ?? "").toLowerCase();
    if (["thinking", "reasoning", "analysis", "redacted_thinking"].includes(kind)) return undefined;
    const result: Record<string, any> = {};
    for (const [key, item] of Object.entries(value)) {
      if (["thinking", "thinking_content", "reasoning", "reasoning_content", "analysis", "chain_of_thought", "thought", "thoughts", "redacted_thinking"].includes(key.toLowerCase())) continue;
      const clean = sanitizeValue(item);
      if (clean !== undefined) result[key] = clean;
    }
    return result;
  }
  return value;
}

function valueText(value: any): string {
  value = sanitizeValue(value);
  if (typeof value === "string") return value.trim();
  if (value === undefined || value === null) return "";
  try { return JSON.stringify(value); } catch { return ""; }
}

function contentMessages(role: string, content: any, source: string): BufferedMessage[] {
  if (role.toLowerCase() === "toolresult") return [];
  if (typeof content === "string") return [{ role, text: content, source }];
  if (!Array.isArray(content)) return [];
  const messages: BufferedMessage[] = [];
  for (const part of content) {
    if (typeof part === "string") {
      messages.push({ role, text: part, source });
      continue;
    }
    const kind = String(part?.type ?? "").toLowerCase();
    if (["thinking", "reasoning", "analysis", "redacted_thinking"].includes(kind)) continue;
    if (["toolcall", "tool_use", "tool_call", "function_call", "toolresult", "tool_result", "tool_response", "function_call_output", "function_result"].includes(kind)) continue;
    const text = valueText(part?.text ?? part?.content);
    if (text !== "") messages.push({ role, text, source });
  }
  return messages;
}

function appendBufferedMessage(role: string, text: string, source: string): void {
  const trimmed = text.trim();
  if (trimmed === "") return;
  const last = turnMessages[turnMessages.length - 1];
  if (last?.role === role && last.text === trimmed) return;
  turnMessages.push({ role, text: trimmed, source });
  if (turnMessages.length > maxBufferedMessages) {
    turnMessages = turnMessages.slice(-maxBufferedMessages);
  }
}

function appendPiMessage(message: any, source: string): void {
  const role = typeof message?.role === "string" ? message.role : "unknown";
  if (role.toLowerCase() === "toolresult") return;
  const messages = contentMessages(role, message?.content, source);
  if (messages.length === 0 && typeof message?.text === "string") {
    appendBufferedMessage(role, message.text, source);
    return;
  }
  for (const item of messages) appendBufferedMessage(item.role, item.text, item.source);
}

function runPaxmHook(command: string, payload: unknown, ctx: any, notifyOnFailure: boolean): { ok: boolean; stdout: string } {
  const result = spawnSync(command, [], {
    input: JSON.stringify(payload) + "\n",
    encoding: "utf8",
    maxBuffer: 1024 * 1024,
  });

  if (result.error) {
    if (notifyOnFailure) ctx?.ui?.notify?.(` + "`" + `paxm hook failed: ${result.error.message}` + "`" + `, "warning");
    return { ok: false, stdout: "" };
  }
  if (result.status !== 0) {
    if (notifyOnFailure) {
      const detail = (result.stderr || result.stdout || "Unknown paxm hook failure.").trim();
      ctx?.ui?.notify?.(` + "`" + `paxm hook failed: ${detail}` + "`" + `, "warning");
    }
    return { ok: false, stdout: result.stdout ?? "" };
  }

  return { ok: true, stdout: result.stdout ?? "" };
}

function flushTurn(triggerEvent: string, event: any, ctx: any): void {
  if (paxmTurnEndHookCommand === "") return;
  const resolvedCtx = activeCtx(ctx);
  const messages = turnMessages;
  if (messages.length === 0 && lastPrompt.trim() === "") return;
  turnMessages = [];

  const payload = {
    schema_version: "paxm.pi.turn_end.v1",
    target: "pi",
    event: "turn_end",
    agent: "pi",
    session_id: currentSessionId(resolvedCtx),
    cwd: currentWorkspace(resolvedCtx),
    workspace: currentWorkspace(resolvedCtx),
    prompt: lastPrompt,
    source: "pi",
    trigger_event: triggerEvent,
    messages,
    metadata: {
      pi_event: triggerEvent,
      message_count: String(messages.length),
    },
  };

  runPaxmHook(paxmTurnEndHookCommand, payload, resolvedCtx, false);
  lastPrompt = "";
}

function formatPaxmRecall(raw: string): string {
  if (raw.trim() === "") return "";
  try {
    const result = JSON.parse(raw);
	const contexts: string[] = [];
	if (typeof result?.additional_context === "string" && result.additional_context.trim() !== "") {
	  contexts.push(result.additional_context.trim());
	}
	if (result?.skipped || !result?.recall?.hits?.length) return contexts.join("\n\n");
    const lines = ["paxm memory recall:"];
    for (const hit of result.recall.hits) {
      const score = typeof hit.score === "number" ? hit.score.toFixed(4) : "n/a";
      const provider = hit.provider ? String(hit.provider) : "unknown";
      const text = hit.text ? escapePaxmRecallText(String(hit.text).trim()) : "";
      if (text === "") continue;
      lines.push("- [" + provider + " score=" + score + "] " + text);
    }
	if (lines.length > 1) contexts.push('<paxm-recall version="1" mode="passive">\n' + lines.join("\n") + "\n</paxm-recall>");
	return contexts.join("\n\n");
  } catch {
    const text = escapePaxmRecallText(raw.trim());
    if (text === "" || text.includes("<paxm-recall") || text.includes("<paxm-local-time") || text.includes("<paxm-session-identity")) return text;
    return '<paxm-recall version="1" mode="passive">\n' + text + "\n</paxm-recall>";
  }
}

function escapePaxmRecallText(text: string): string {
  return text
    .split("</paxm-recall>").join("&lt;/paxm-recall&gt;")
    .split("<paxm-recall").join("&lt;paxm-recall");
}

export default function (pi: ExtensionAPI) {
  const onRuntimeEvent = pi.on as unknown as (event: string, handler: (event: any, ctx: any) => unknown) => void;

  onRuntimeEvent("session_start", (_event, ctx) => {
    activeContext = ctx;
    lastPrompt = "";
    turnMessages = [];
    pendingToolArgs.clear();
    pendingSessionContext = "";
    if (paxmSessionStartHookCommand !== "") {
      const resolvedCtx = activeCtx(ctx);
      const result = runPaxmHook(paxmSessionStartHookCommand, {
        schema_version: "paxm.pi.session_start.v1",
        target: "pi",
        event: "session_start",
        agent: "pi",
        session_id: currentSessionId(resolvedCtx),
        cwd: currentWorkspace(resolvedCtx),
        workspace: currentWorkspace(resolvedCtx),
        source: "pi",
      }, resolvedCtx, false);
      if (result.ok) pendingSessionContext = result.stdout.trim();
    }
  });

  onRuntimeEvent("message_end", (event, ctx) => {
    if (paxmTurnEndHookCommand === "") return;
    activeCtx(ctx);
    appendPiMessage(event?.message, "message_end");
  });

  onRuntimeEvent("tool_execution_start", (event, ctx) => {
    if (paxmTurnEndHookCommand === "") return;
    activeCtx(ctx);
    const toolCallId = valueText(event?.toolCallId);
    if (toolCallId !== "") pendingToolArgs.set(toolCallId, event?.args);
  });

  onRuntimeEvent("tool_execution_end", (event, ctx) => {
    if (paxmTurnEndHookCommand === "") return;
    activeCtx(ctx);
    const name = valueText(event?.toolName);
    const toolCallId = valueText(event?.toolCallId);
    const args = valueText(pendingToolArgs.get(toolCallId));
    if (toolCallId !== "") pendingToolArgs.delete(toolCallId);
    appendBufferedMessage("tool_call", [name, args].filter(Boolean).join(" "), "tool_execution_end");
    const result = valueText(event?.result);
    if (result !== "") appendBufferedMessage("tool_result", event?.isError ? "Error: " + result : result, "tool_execution_end");
  });

  onRuntimeEvent("agent_end", (event, ctx) => {
    if (paxmTurnEndHookCommand === "") return;
    flushTurn("agent_end", event, ctx);
    pendingToolArgs.clear();
  });

  onRuntimeEvent("session_shutdown", (event, ctx) => {
    if (paxmTurnEndHookCommand === "") return;
    flushTurn("session_shutdown", event, ctx);
    lastPrompt = "";
    turnMessages = [];
    pendingToolArgs.clear();
  });

  pi.on("before_agent_start", async (event, ctx) => {
    const resolvedCtx = activeCtx(ctx);
    lastPrompt = typeof event.prompt === "string" ? event.prompt : "";
    if (paxmTurnEndHookCommand !== "") {
      appendBufferedMessage("user", lastPrompt, "before_agent_start");
    }
    let recallContext = "";

	if (paxmUserInputHookCommand !== "") {
      const payload = {
        schema_version: "paxm.pi.user_input.v1",
        target: "pi",
        event: "user_input",
        agent: "pi",
        session_id: currentSessionId(resolvedCtx),
        cwd: currentWorkspace(resolvedCtx),
        workspace: currentWorkspace(resolvedCtx),
        prompt: event.prompt,
        source: "pi",
      };
      const result = runPaxmHook(paxmUserInputHookCommand, payload, resolvedCtx, true);
      if (result.ok) recallContext = formatPaxmRecall(result.stdout);
    }
    const content = [pendingSessionContext, recallContext].filter(Boolean).join("\n\n");
    pendingSessionContext = "";
    if (content === "") return;

    return {
      message: {
        customType: "paxm-memory-recall",
        content,
        display: true,
        details: {
          source: "paxm",
          event: "user_input",
        },
      },
    };
  });
}
`
}

func jsonStringLiteral(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(encoded)
}

func installCodexGlobalHook(path, scriptPath, configEvent string) error {
	path = config.ExpandPath(path)
	installEvent, ok := hookInstallEventByConfig(configEvent)
	if !ok {
		return fmt.Errorf("unsupported Codex hook event %q", configEvent)
	}
	command := shellQuote(scriptPath)
	commandHook := `{ type = "command", command = "` + escapeTomlString(command) + `", async = false, statusMessage = "` + escapeTomlString(installEvent.Status) + `" }`
	entry := `{ hooks = [` + commandHook + `] }`
	if installEvent.Matcher != "" {
		entry = `{ matcher = "` + escapeTomlString(installEvent.Matcher) + `", hooks = [` + commandHook + `] }`
	}

	contentBytes, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	content, prunedLegacy := pruneLegacyCodexUserPromptHook(string(contentBytes))
	if strings.Contains(content, scriptPath) || strings.Contains(content, command) {
		if prunedLegacy {
			return writeCodexConfig(path, contentBytes, content)
		}
		return nil
	}

	updated := upsertCodexHook(content, installEvent.NativeEvent, entry)
	return writeCodexConfig(path, contentBytes, updated)
}

type claudeHookHandler struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

type claudeHookGroup struct {
	Matcher string              `json:"matcher,omitempty"`
	Hooks   []claudeHookHandler `json:"hooks"`
}

func installClaudeGlobalHooks(path string, scriptPaths map[string]string) error {
	path = config.ExpandPath(path)
	original, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	settings := make(map[string]json.RawMessage)
	if len(bytesTrimSpace(original)) > 0 {
		if err := json.Unmarshal(original, &settings); err != nil {
			return fmt.Errorf("decode Claude Code settings %s: %w", path, err)
		}
	}
	hooks := make(map[string][]json.RawMessage)
	if rawHooks := settings["hooks"]; len(bytesTrimSpace(rawHooks)) > 0 && string(bytesTrimSpace(rawHooks)) != "null" {
		if err := json.Unmarshal(rawHooks, &hooks); err != nil {
			return fmt.Errorf("decode Claude Code hooks %s: %w", path, err)
		}
	}
	changed := false
	hasHook := false
	for _, installEvent := range installedHookEvents() {
		scriptPath := strings.TrimSpace(scriptPaths[installEvent.ConfigEvent])
		if scriptPath == "" {
			continue
		}
		hasHook = true
		command := nativeHookCommand(scriptPath)
		alreadyInstalled := false
		for _, rawGroup := range hooks[installEvent.NativeEvent] {
			if claudeHookGroupHasCommand(rawGroup, command, scriptPath) {
				alreadyInstalled = true
				break
			}
		}
		if alreadyInstalled {
			continue
		}
		group := claudeHookGroup{
			Matcher: installEvent.Matcher,
			Hooks: []claudeHookHandler{{
				Type:    "command",
				Command: command,
				Timeout: 60,
			}},
		}
		groupBytes, err := json.Marshal(group)
		if err != nil {
			return err
		}
		hooks[installEvent.NativeEvent] = append(hooks[installEvent.NativeEvent], groupBytes)
		changed = true
	}
	if !hasHook {
		return errors.New("Claude Code hook requires at least one hook shim")
	}
	if !changed {
		return nil
	}
	hooksBytes, err := json.Marshal(hooks)
	if err != nil {
		return err
	}
	settings["hooks"] = hooksBytes
	return writeClaudeSettings(path, original, settings)
}

func claudeHookGroupHasCommand(rawGroup json.RawMessage, command, scriptPath string) bool {
	var group struct {
		Hooks []struct {
			Command string `json:"command"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(rawGroup, &group); err != nil {
		return false
	}
	for _, hook := range group.Hooks {
		if hook.Command == command || hook.Command == scriptPath || strings.Contains(hook.Command, scriptPath) {
			return true
		}
	}
	return false
}

func writeClaudeSettings(path string, original []byte, settings map[string]json.RawMessage) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if len(original) > 0 {
		backupPath := path + ".paxm.bak"
		if _, err := os.Stat(backupPath); errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(backupPath, original, 0o600); err != nil {
				return err
			}
		}
	}
	updated, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	updated = append(updated, '\n')
	return os.WriteFile(path, updated, 0o600)
}

func writeCodexConfig(path string, original []byte, updated string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if len(original) > 0 {
		backupPath := path + ".paxm.bak"
		if _, err := os.Stat(backupPath); errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(backupPath, original, 0o600); err != nil {
				return err
			}
		}
	}
	return os.WriteFile(path, []byte(updated), 0o600)
}

func pruneLegacyCodexUserPromptHook(content string) (string, bool) {
	if !strings.Contains(content, "codex-user_prompt") {
		return content, false
	}
	lines := strings.SplitAfter(content, "\n")
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "UserPromptSubmit = ") {
			next := removeInlineTomlArrayEntries(line, "codex-user_prompt")
			if next != line {
				lines[i] = next
				changed = true
			}
		}
	}
	if !changed {
		return content, false
	}
	return strings.Join(lines, ""), true
}

func upsertCodexHook(content, eventName, entry string) string {
	if content == "" {
		return "[hooks]\n" + eventName + " = [" + entry + "]\n"
	}
	lines := strings.SplitAfter(content, "\n")
	hooksStart := -1
	hooksEnd := len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[hooks]" {
			hooksStart = i
			continue
		}
		if hooksStart != -1 && i > hooksStart && strings.HasPrefix(trimmed, "[") {
			hooksEnd = i
			break
		}
	}
	if hooksStart == -1 {
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		return content + "\n[hooks]\n" + eventName + " = [" + entry + "]\n"
	}
	for i := hooksStart + 1; i < hooksEnd; i++ {
		line := lines[i]
		if strings.HasPrefix(strings.TrimSpace(line), eventName+" = ") {
			lines[i] = appendInlineTomlArray(line, entry)
			return strings.Join(lines, "")
		}
	}
	newLine := eventName + " = [" + entry + "]\n"
	updated := append([]string{}, lines[:hooksStart+1]...)
	updated = append(updated, newLine)
	updated = append(updated, lines[hooksStart+1:]...)
	return strings.Join(updated, "")
}

func removeInlineTomlArrayEntries(line, marker string) string {
	newline := ""
	if strings.HasSuffix(line, "\n") {
		newline = "\n"
		line = strings.TrimSuffix(line, "\n")
	}
	start := strings.Index(line, "[")
	end := strings.LastIndex(line, "]")
	if start == -1 || end <= start {
		return line + newline
	}
	prefix := line[:start+1]
	body := line[start+1 : end]
	suffix := line[end:]
	entries := splitTopLevelInlineEntries(body)
	filtered := entries[:0]
	changed := false
	for _, entry := range entries {
		if strings.Contains(entry, marker) {
			changed = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if !changed {
		return line + newline
	}
	return prefix + strings.Join(filtered, ", ") + suffix + newline
}

func splitTopLevelInlineEntries(body string) []string {
	var entries []string
	start := 0
	depth := 0
	inString := false
	escaped := false
	for i, char := range body {
		if escaped {
			escaped = false
			continue
		}
		if inString {
			if char == '\\' {
				escaped = true
				continue
			}
			if char == '"' {
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '{', '[':
			depth++
		case '}', ']':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				entries = append(entries, strings.TrimSpace(body[start:i]))
				start = i + 1
			}
		}
	}
	if strings.TrimSpace(body[start:]) != "" {
		entries = append(entries, strings.TrimSpace(body[start:]))
	}
	return entries
}

func appendInlineTomlArray(line, entry string) string {
	newline := ""
	if strings.HasSuffix(line, "\n") {
		newline = "\n"
		line = strings.TrimSuffix(line, "\n")
	}
	index := strings.LastIndex(line, "]")
	if index == -1 {
		return line + newline
	}
	prefix := strings.TrimRight(line[:index], " ")
	suffix := line[index:]
	if strings.HasSuffix(prefix, "[") {
		return prefix + entry + suffix + newline
	}
	return prefix + ", " + entry + suffix + newline
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func escapeTomlString(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return replacer.Replace(value)
}
