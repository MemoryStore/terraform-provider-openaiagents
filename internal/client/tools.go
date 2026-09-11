// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

var supportedToolTypes = map[string]struct{}{
	"function":                  {},
	"tool_search":               {},
	"programmatic_tool_calling": {},
	"mcp":                       {},
	"web_search":                {},
}

var secretHeaderNames = map[string]struct{}{
	"authorization":       {},
	"proxy-authorization": {},
	"cookie":              {},
	"set-cookie":          {},
	"x-api-key":           {},
	"x-auth-token":        {},
	"x-openai-api-key":    {},
}

// ValidatePersistedTools rejects unsupported variants and credential-bearing MCP fields.
func ValidatePersistedTools(tools []json.RawMessage) error {
	for i, raw := range tools {
		if err := validatePersistedTool(raw); err != nil {
			return fmt.Errorf("tools[%d]: %w", i, err)
		}
	}
	return nil
}

func validatePersistedTool(raw json.RawMessage) error {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return fmt.Errorf("invalid tool object: %w", err)
	}
	if probe.Type == "" {
		return fmt.Errorf("tool type is required")
	}
	if _, ok := supportedToolTypes[probe.Type]; !ok {
		return fmt.Errorf("unsupported tool type %q; supported: function, tool_search, programmatic_tool_calling, mcp, web_search", probe.Type)
	}
	if probe.Type != "mcp" {
		return nil
	}

	var mcp map[string]json.RawMessage
	if err := json.Unmarshal(raw, &mcp); err != nil {
		return fmt.Errorf("invalid mcp tool: %w", err)
	}
	transportRaw, ok := mcp["transport"]
	if !ok {
		return fmt.Errorf("mcp tool requires transport")
	}
	var transport map[string]json.RawMessage
	if err := json.Unmarshal(transportRaw, &transport); err != nil {
		return fmt.Errorf("invalid mcp transport: %w", err)
	}
	if _, exists := transport["authorization"]; exists {
		return fmt.Errorf("saved agents must not include inline MCP authorization; use a vault credential reference")
	}
	if headersRaw, exists := transport["headers"]; exists && string(headersRaw) != "null" {
		var headers map[string]string
		if err := json.Unmarshal(headersRaw, &headers); err != nil {
			return fmt.Errorf("invalid mcp headers: %w", err)
		}
		for key, value := range headers {
			if err := rejectSecretHeader(key, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func rejectSecretHeader(key, value string) error {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if _, forbidden := secretHeaderNames[normalized]; forbidden {
		return fmt.Errorf("saved agents must not include credential-bearing header %q", key)
	}
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "bearer ") || strings.HasPrefix(lower, "basic ") {
		return fmt.Errorf("saved agents must not include credential-bearing header values")
	}
	return nil
}

// ToolType returns the type field of a persisted tool object.
func ToolType(raw json.RawMessage) (string, error) {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return "", err
	}
	return probe.Type, nil
}
