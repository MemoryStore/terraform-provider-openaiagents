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
	// Never forward remote bodies. They can echo tokens, setup-command
	// bodies, or archive fragments. Callers still see HTTP status and request_id.
	_ = message
	return "API request failed"
}

// MissingAPIKeySummary and MissingAPIKeyDetail are the diagnostic strings a
// caller should surface when a request is attempted without a resolved key.
// The wording is deliberately identical to the text the provider used to emit
// from Configure so existing log searches keep matching.
const (
	MissingAPIKeySummary = "Missing API key"
	MissingAPIKeyDetail  = "Set the provider argument api_key or the OPENAI_API_KEY environment variable. This provider does not use OPENAI_ADMIN_KEY."
)

// MissingAPIKeyError reports a request that was refused before it was sent
// because the client carries no API key. Credentials are checked here, at
// first use, rather than during provider configuration, so a configuration
// that declares the provider without enabling any resource can still plan.
type MissingAPIKeyError struct{}

func (e *MissingAPIKeyError) Error() string {
	return MissingAPIKeySummary + ": " + MissingAPIKeyDetail
}

// IsMissingAPIKey reports whether err is the client's missing-credential error.
func IsMissingAPIKey(err error) bool {
	var missing *MissingAPIKeyError
	return errors.As(err, &missing)
}
