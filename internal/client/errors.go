// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// APIError is a redacted API or transport failure.
type APIError struct {
	Operation  string
	Resource   string
	ID         string
	StatusCode int
	RequestID  string
	Message    string
}

func (e *APIError) Error() string {
	var b strings.Builder
	b.WriteString(e.Operation)
	if e.Resource != "" {
		b.WriteString(" ")
		b.WriteString(e.Resource)
	}
	if e.ID != "" {
		b.WriteString(" ")
		b.WriteString(e.ID)
	}
	if e.StatusCode != 0 {
		fmt.Fprintf(&b, ": HTTP %d", e.StatusCode)
	}
	if e.RequestID != "" {
		fmt.Fprintf(&b, " request_id=%s", e.RequestID)
	}
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}
	return b.String()
}

// IsNotFound reports whether err is an HTTP 404 from this client.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

func newAPIError(operation, resource, id string, status int, requestID, message string) *APIError {
	return &APIError{
		Operation:  operation,
		Resource:   resource,
		ID:         id,
		StatusCode: status,
		RequestID:  requestID,
		Message:    sanitizeErrorMessage(message),
	}
}

func sanitizeErrorMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	lower := strings.ToLower(message)
	for _, needle := range []string{
		"bearer ",
		"sk-",
		"authorization",
		"client_secret",
		"refresh_token",
		"access_token",
	} {
		if strings.Contains(lower, needle) {
			return "API request failed"
		}
	}
	if len(message) > 300 {
		return message[:300]
	}
	return message
}
