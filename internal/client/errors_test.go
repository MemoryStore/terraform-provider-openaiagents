// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"strings"
	"testing"
)

func TestSanitizeErrorMessageNeverEchoesBody(t *testing.T) {
	err := newAPIError("CreateAgent", "agent", "id", 400, "req_1", "setup failed: CANARY_SECRET_DO_NOT_LEAK")
	if strings.Contains(err.Error(), "CANARY_SECRET_DO_NOT_LEAK") {
		t.Fatalf("secret leaked in diagnostics: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "req_1") || !strings.Contains(err.Error(), "400") {
		t.Fatalf("expected status and request id, got %s", err.Error())
	}
	if !strings.Contains(err.Error(), "API request failed") {
		t.Fatalf("expected generic message, got %s", err.Error())
	}
}
