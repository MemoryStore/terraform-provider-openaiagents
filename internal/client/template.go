// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"net/http"
)

// CreateTemplate POSTs /agents/environments/templates.
func (c *Client) CreateTemplate(ctx context.Context, write TemplateWrite) (*EnvironmentTemplate, error) {
	write.SendConfidential = true
	rawURL, err := c.url("agents", "environments", "templates")
	if err != nil {
		return nil, err
	}
	var out EnvironmentTemplate
	if err := c.doJSON(ctx, http.MethodPost, rawURL, "CreateTemplate", "environment_template", "", write.toMap(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTemplate GETs /agents/environments/templates/{id}.
func (c *Client) GetTemplate(ctx context.Context, id string) (*EnvironmentTemplate, error) {
	rawURL, err := c.url("agents", "environments", "templates", id)
	if err != nil {
		return nil, err
	}
	var out EnvironmentTemplate
	if err := c.doJSON(ctx, http.MethodGet, rawURL, "GetTemplate", "environment_template", id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateTemplate POSTs /agents/environments/templates/{id}.
func (c *Client) UpdateTemplate(ctx context.Context, id string, write TemplateWrite) (*EnvironmentTemplate, error) {
	rawURL, err := c.url("agents", "environments", "templates", id)
	if err != nil {
		return nil, err
	}
	var out EnvironmentTemplate
	if err := c.doJSON(ctx, http.MethodPost, rawURL, "UpdateTemplate", "environment_template", id, write.toMap(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteTemplate DELETEs /agents/environments/templates/{id}. A 404 is success.
func (c *Client) DeleteTemplate(ctx context.Context, id string) error {
	rawURL, err := c.url("agents", "environments", "templates", id)
	if err != nil {
		return err
	}
	var out Deleted
	err = c.doJSON(ctx, http.MethodDelete, rawURL, "DeleteTemplate", "environment_template", id, nil, &out)
	if IsNotFound(err) {
		return nil
	}
	return err
}
