// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// CanonicalJSON returns a stable JSON encoding: object keys sorted, array order
// preserved, boolean and numeric values unchanged. Nested JSON Schema keywords
// such as additionalProperties: false are preserved.
func CanonicalJSON(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	var value any
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	out, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// EqualCanonicalJSON reports whether two JSON documents are semantically equal
// after canonicalization.
func EqualCanonicalJSON(a, b string) (bool, error) {
	ca, err := CanonicalJSON(a)
	if err != nil {
		return false, err
	}
	cb, err := CanonicalJSON(b)
	if err != nil {
		return false, err
	}
	return ca == cb, nil
}
