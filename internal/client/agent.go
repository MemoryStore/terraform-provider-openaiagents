// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"net/http"
)

// CreateAgent POSTs /agents.
func (c *Client) CreateAgent(ctx context.Context, write AgentWrite) (*Agent, error) {
	if write.Tools.Present && !write.Tools.Null {
		if err := ValidatePersistedTools(write.Tools.Value); err != nil {
			return nil, err
		}
	}
	rawURL, err := c.url("agents")
	if err != nil {
		return nil, err
	}
	var out Agent
	if err := c.doJSON(ctx, http.MethodPost, rawURL, "CreateAgent", "agent", "", write.toMap(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAgent GETs /agents/{id}.
func (c *Client) GetAgent(ctx context.Context, id string) (*Agent, error) {
	rawURL, err := c.url("agents", id)
	if err != nil {
		return nil, err
	}
	var out Agent
	if err := c.doJSON(ctx, http.MethodGet, rawURL, "GetAgent", "agent", id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateAgent POSTs /agents/{id}. Omitted Optional fields are not sent.
func (c *Client) UpdateAgent(ctx context.Context, id string, write AgentWrite) (*Agent, error) {
	if write.Tools.Present && !write.Tools.Null {
		if err := ValidatePersistedTools(write.Tools.Value); err != nil {
			return nil, err
		}
	}
	rawURL, err := c.url("agents", id)
	if err != nil {
		return nil, err
	}
	var out Agent
	if err := c.doJSON(ctx, http.MethodPost, rawURL, "UpdateAgent", "agent", id, write.toMap(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteAgent DELETEs /agents/{id}. A 404 is success.
func (c *Client) DeleteAgent(ctx context.Context, id string) error {
	rawURL, err := c.url("agents", id)
	if err != nil {
		return err
	}
	var out Deleted
	err = c.doJSON(ctx, http.MethodDelete, rawURL, "DeleteAgent", "agent", id, nil, &out)
	if IsNotFound(err) {
		return nil
	}
	return err
}
