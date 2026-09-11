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
	var transportType string
	if tRaw, exists := transport["type"]; exists {
		_ = json.Unmarshal(tRaw, &transportType)
	}
	switch transportType {
	case "http":
		var serverURL string
		if uRaw, exists := transport["server_url"]; exists {
			_ = json.Unmarshal(uRaw, &serverURL)
		}
		if strings.TrimSpace(serverURL) == "" {
			return fmt.Errorf("mcp http transport requires server_url")
		}
	case "stdio":
		var command, cwd string
		if cRaw, exists := transport["command"]; exists {
			_ = json.Unmarshal(cRaw, &command)
		}
		if dRaw, exists := transport["cwd"]; exists {
			_ = json.Unmarshal(dRaw, &cwd)
		}
		if strings.TrimSpace(command) == "" || strings.TrimSpace(cwd) == "" {
			return fmt.Errorf("mcp stdio transport requires command and cwd")
		}
	case "":
		return fmt.Errorf("mcp transport type is required")
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
	if metaRaw, exists := mcp["request_metadata"]; exists {
		if err := rejectSecretJSON(metaRaw); err != nil {
			return fmt.Errorf("mcp request_metadata: %w", err)
		}
	}
	return nil
}

func rejectSecretHeader(key, value string) error {
	if headerLooksSecret(key, value) {
		return fmt.Errorf("saved agents must not include credential-bearing header %q", key)
	}
	return nil
}

func headerLooksSecret(key, value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	if _, forbidden := secretHeaderNames[normalized]; forbidden {
		return true
	}
	if strings.Contains(normalized, "authorization") {
		return true
	}
	if strings.Contains(normalized, "secret") || strings.Contains(normalized, "password") || strings.Contains(normalized, "passwd") {
		return true
	}
	hasAPI := strings.Contains(normalized, "api")
	hasKey := strings.Contains(normalized, "key")
	hasToken := strings.Contains(normalized, "token")
	if hasAPI && (hasKey || hasToken) {
		return true
	}
	if hasKey && strings.HasPrefix(normalized, "x-") {
		return true
	}
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if strings.HasPrefix(trimmed, "bearer ") || strings.HasPrefix(trimmed, "basic ") {
		return true
	}
	if strings.HasPrefix(trimmed, "sk-") {
		return true
	}
	return false
}

// ScanSecretsInJSON rejects credential-bearing keys or values in arbitrary JSON.
func ScanSecretsInJSON(raw json.RawMessage) error {
	return rejectSecretJSON(raw)
}

func rejectSecretJSON(raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return walkSecretJSON(value, "")
}

func walkSecretJSON(value any, path string) error {
	switch v := value.(type) {
	case map[string]any:
		for k, child := range v {
			if headerLooksSecret(k, stringifyJSON(child)) {
				return fmt.Errorf("saved agents must not include credential-bearing field %q", k)
			}
			if err := walkSecretJSON(child, path+"."+k); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range v {
			if err := walkSecretJSON(child, path); err != nil {
				return err
			}
		}
	}
	return nil
}

func stringifyJSON(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
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
