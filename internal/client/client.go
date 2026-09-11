// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Options configures an Agents API client.
type Options struct {
	APIKey          string
	Organization    string
	Project         string
	BaseURL         string
	HTTPClient      *http.Client
	UserAgent       string
	AllowProduction bool
	Timeout         time.Duration
}

// Client is a typed HTTP adapter for the hosted OpenAI Agents API.
type Client struct {
	apiKey          string
	organization    string
	project         string
	baseURL         *url.URL
	http            *http.Client
	userAgent       string
	allowProduction bool
}

// New constructs a Client. It never reads OPENAI_ADMIN_KEY.
func New(opts Options) (*Client, error) {
	if strings.TrimSpace(opts.APIKey) == "" {
		return nil, fmt.Errorf("api_key is required")
	}

	base := opts.BaseURL
	if strings.TrimSpace(base) == "" {
		base = defaultBaseURL
	}
	endpoint, err := validateAPIBaseURL(base)
	if err != nil {
		return nil, err
	}
	endpoint.Path = strings.TrimSuffix(endpoint.Path, "/")

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = newCredentialAudienceHTTPClient(endpoint, opts.Timeout)
	}

	ua := opts.UserAgent
	if ua == "" {
		ua = userAgentName
	}

	return &Client{
		apiKey:          opts.APIKey,
		organization:    opts.Organization,
		project:         opts.Project,
		baseURL:         endpoint,
		http:            httpClient,
		userAgent:       ua,
		allowProduction: opts.AllowProduction,
	}, nil
}

func (c *Client) url(parts ...string) (string, error) {
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		if err := validatePathSegment(part); err != nil {
			return "", err
		}
		escaped = append(escaped, url.PathEscape(part))
	}
	rel := strings.Join(escaped, "/")
	u, err := url.Parse(c.baseURL.String() + "/" + rel)
	if err != nil {
		return "", err
	}
	if !c.allowProduction && isProductionHost(u.Hostname()) {
		return "", fmt.Errorf("refusing to contact production OpenAI API host %q", u.Hostname())
	}
	return u.String(), nil
}

func validatePathSegment(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("resource id is empty")
	}
	if strings.ContainsAny(id, "/?#") || strings.Contains(id, "..") {
		return fmt.Errorf("malformed resource id")
	}
	return nil
}

func (c *Client) doJSON(ctx context.Context, method, rawURL, operation, resource, id string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("%s: encode request: %w", operation, err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("OpenAI-Beta", betaHeaderValue)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.organization != "" {
		req.Header.Set("OpenAI-Organization", c.organization)
	}
	if c.project != "" {
		req.Header.Set("OpenAI-Project", c.project)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", operation, resource, err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return newAPIError(operation, resource, id, resp.StatusCode, resp.Header.Get("X-Request-ID"), "failed to read response")
	}
	if int64(len(data)) > maxResponseBytes {
		return newAPIError(operation, resource, id, resp.StatusCode, resp.Header.Get("X-Request-ID"), "response exceeded size limit")
	}

	requestID := resp.Header.Get("X-Request-ID")
	if resp.StatusCode >= 400 {
		return newAPIError(operation, resource, id, resp.StatusCode, requestID, extractAPIMessage(data))
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return newAPIError(operation, resource, id, resp.StatusCode, requestID, "empty response body")
	}
	if err := json.Unmarshal(data, out); err != nil {
		return newAPIError(operation, resource, id, resp.StatusCode, requestID, "malformed JSON response")
	}
	return nil
}

func extractAPIMessage(data []byte) string {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Error.Message != "" {
		return envelope.Error.Message
	}
	return "API request failed"
}

// Deleted is a delete acknowledgement.
type Deleted struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}
