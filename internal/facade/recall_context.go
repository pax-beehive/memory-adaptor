package facade

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

const (
	recallEnvelopeOpenPrefix = "<paxm-recall"
	recallEnvelopeClose      = "</paxm-recall>"
)

func WrapRecallContext(mode, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "passive" && mode != "active" {
		mode = "unknown"
	}
	text = strings.ReplaceAll(text, recallEnvelopeClose, "&lt;/paxm-recall&gt;")
	text = strings.ReplaceAll(text, recallEnvelopeOpenPrefix, "&lt;paxm-recall")
	return recallEnvelopeOpenPrefix + ` version="1" mode="` + mode + `">` + "\n" + text + "\n" + recallEnvelopeClose
}

func StripRecallContext(text string) string {
	var cleaned strings.Builder
	remaining := text
	for {
		start := strings.Index(remaining, recallEnvelopeOpenPrefix)
		if start < 0 {
			cleaned.WriteString(remaining)
			break
		}
		openEnd := strings.Index(remaining[start:], ">")
		if openEnd < 0 {
			cleaned.WriteString(remaining)
			break
		}
		openEnd += start
		closeStart := strings.Index(remaining[openEnd+1:], recallEnvelopeClose)
		if closeStart < 0 {
			cleaned.WriteString(remaining)
			break
		}
		closeEnd := openEnd + 1 + closeStart + len(recallEnvelopeClose)
		cleaned.WriteString(remaining[:start])
		remaining = remaining[closeEnd:]
	}
	return strings.TrimSpace(cleaned.String())
}

func stripRecallContextFromHookEvent(event HookEvent) HookEvent {
	event.Query = StripRecallContext(event.Query)
	event.Prompt = StripRecallContext(event.Prompt)
	event.Assistant = StripRecallContext(event.Assistant)
	for i := range event.Messages {
		event.Messages[i].Text = StripRecallContext(event.Messages[i].Text)
		event.Messages[i].Content = StripRecallContext(event.Messages[i].Content)
	}
	event.Messages = dropPaxmRecallToolExchanges(event.Messages)
	return event
}

func dropPaxmRecallToolExchanges(messages []HookMessage) []HookMessage {
	filtered := make([]HookMessage, 0, len(messages))
	dropResults := false
	for _, message := range messages {
		role := normalizeHookMessageRole(message.Role)
		if role == "tool_result" && isPaxmRecallToolResult(message) {
			continue
		}
		if role == "tool_call" {
			dropResults = isPaxmRecallToolCall(message)
			if dropResults {
				continue
			}
			filtered = append(filtered, message)
			continue
		}
		if role == "tool_result" && dropResults {
			continue
		}
		dropResults = false
		filtered = append(filtered, message)
	}
	return filtered
}

func isPaxmRecallToolResult(message HookMessage) bool {
	text := strings.TrimSpace(firstNonEmpty(message.Text, message.Content))
	if text == "" {
		return false
	}
	var value struct {
		PaxmContext struct {
			Kind string `json:"kind"`
		} `json:"paxm_context"`
	}
	return json.Unmarshal([]byte(text), &value) == nil && value.PaxmContext.Kind == "recall"
}

func isPaxmRecallToolCall(message HookMessage) bool {
	text := strings.TrimSpace(firstNonEmpty(message.Text, message.Content))
	source := strings.ToLower(strings.TrimSpace(message.Source))
	if isPaxmRecallToolName(source) {
		return true
	}
	name, arguments, _ := strings.Cut(text, " ")
	if isPaxmRecallToolName(strings.ToLower(strings.TrimSpace(name))) {
		return true
	}
	if strings.EqualFold(filepath.Base(strings.TrimSpace(name)), "bash") {
		var input struct {
			Command string `json:"command"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(arguments)), &input) != nil {
			return false
		}
		return isPaxmRecallCommand(input.Command)
	}
	return isPaxmRecallCommand(text)
}

func isPaxmRecallToolName(name string) bool {
	return name == "paxm_recall" || strings.HasSuffix(name, "__paxm_recall")
}

func isPaxmRecallCommand(command string) bool {
	words := strings.Fields(command)
	if len(words) == 0 {
		return false
	}
	index := 0
	if filepath.Base(strings.Trim(words[index], `"'`)) == "env" {
		index++
	}
	for index < len(words) && strings.Contains(words[index], "=") && !strings.HasPrefix(words[index], "-") {
		index++
	}
	if index >= len(words) || filepath.Base(strings.Trim(words[index], `"'`)) != "paxm" {
		return false
	}
	for index++; index < len(words); index++ {
		word := strings.Trim(words[index], `"'`)
		if word == "--config" {
			index++
			continue
		}
		if strings.HasPrefix(word, "--config=") {
			continue
		}
		if strings.HasPrefix(word, "-") {
			continue
		}
		return word == "recall"
	}
	return false
}
