package config

import (
	"errors"
	"strings"
)

type Mem0SearchScopePayload string

const (
	Mem0SearchScopePayloadAuto     Mem0SearchScopePayload = "auto"
	Mem0SearchScopePayloadFilters  Mem0SearchScopePayload = "filters"
	Mem0SearchScopePayloadTopLevel Mem0SearchScopePayload = "top_level"
)

func NormalizeMem0SearchScopePayload(value string) Mem0SearchScopePayload {
	if strings.TrimSpace(value) == "" {
		return Mem0SearchScopePayloadAuto
	}
	return Mem0SearchScopePayload(strings.ToLower(strings.TrimSpace(value)))
}

func ParseMem0SearchScopePayload(value string) (Mem0SearchScopePayload, error) {
	payload := NormalizeMem0SearchScopePayload(value)
	switch payload {
	case Mem0SearchScopePayloadAuto, Mem0SearchScopePayloadFilters, Mem0SearchScopePayloadTopLevel:
		return payload, nil
	default:
		return "", errors.New("search_scope_payload must be auto, filters, or top_level")
	}
}
