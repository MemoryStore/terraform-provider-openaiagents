// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"net/http"
)

// CreateVault POSTs /vaults. There is no vault update API.
func (c *Client) CreateVault(ctx context.Context, write VaultWrite) (*Vault, error) {
	rawURL, err := c.url("vaults")
	if err != nil {
		return nil, err
	}
	body := map[string]any{}
	write.Name.apply(body, "name")
	write.Metadata.apply(body, "metadata")
	var out Vault
	if err := c.doJSON(ctx, http.MethodPost, rawURL, "CreateVault", "vault", "", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetVault GETs /vaults/{id}.
func (c *Client) GetVault(ctx context.Context, id string) (*Vault, error) {
	rawURL, err := c.url("vaults", id)
	if err != nil {
		return nil, err
	}
	var out Vault
	if err := c.doJSON(ctx, http.MethodGet, rawURL, "GetVault", "vault", id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteVault DELETEs /vaults/{id}. A 404 is success. The API removes the vault
// and its credentials; this client does not enumerate children.
func (c *Client) DeleteVault(ctx context.Context, id string) error {
	rawURL, err := c.url("vaults", id)
	if err != nil {
		return err
	}
	var out Deleted
	err = c.doJSON(ctx, http.MethodDelete, rawURL, "DeleteVault", "vault", id, nil, &out)
	if IsNotFound(err) {
		return nil
	}
	return err
}

// CreateCredential POSTs /vaults/{vault_id}/credentials.
func (c *Client) CreateCredential(ctx context.Context, vaultID string, write CredentialCreate) (*Credential, error) {
	rawURL, err := c.url("vaults", vaultID, "credentials")
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"name": write.Name,
		"auth": credentialAuthCreateMap(write.Auth),
	}
	var out Credential
	if err := c.doJSON(ctx, http.MethodPost, rawURL, "CreateCredential", "vault_credential", "", body, &out); err != nil {
		return nil, err
	}
	if out.VaultID == "" {
		out.VaultID = vaultID
	}
	return &out, nil
}

// GetCredential GETs /vaults/{vault_id}/credentials/{id}.
func (c *Client) GetCredential(ctx context.Context, vaultID, id string) (*Credential, error) {
	rawURL, err := c.url("vaults", vaultID, "credentials", id)
	if err != nil {
		return nil, err
	}
	var out Credential
	if err := c.doJSON(ctx, http.MethodGet, rawURL, "GetCredential", "vault_credential", id, nil, &out); err != nil {
		return nil, err
	}
	if out.VaultID == "" {
		out.VaultID = vaultID
	}
	return &out, nil
}

// RotateCredential POSTs /vaults/{vault_id}/credentials/{id}.
func (c *Client) RotateCredential(ctx context.Context, vaultID, id string, rotate CredentialRotate) (*Credential, error) {
	rawURL, err := c.url("vaults", vaultID, "credentials", id)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"auth": credentialRotateMap(rotate),
	}
	var out Credential
	if err := c.doJSON(ctx, http.MethodPost, rawURL, "RotateCredential", "vault_credential", id, body, &out); err != nil {
		return nil, err
	}
	if out.VaultID == "" {
		out.VaultID = vaultID
	}
	return &out, nil
}

// DeleteCredential DELETEs /vaults/{vault_id}/credentials/{id}. A 404 is success.
func (c *Client) DeleteCredential(ctx context.Context, vaultID, id string) error {
	rawURL, err := c.url("vaults", vaultID, "credentials", id)
	if err != nil {
		return err
	}
	var out Deleted
	err = c.doJSON(ctx, http.MethodDelete, rawURL, "DeleteCredential", "vault_credential", id, nil, &out)
	if IsNotFound(err) {
		return nil
	}
	return err
}

func credentialAuthCreateMap(auth CredentialAuthWrite) map[string]any {
	body := map[string]any{
		"type":           auth.Type,
		"mcp_server_url": auth.MCPServerURL,
	}
	switch auth.Type {
	case "static_bearer":
		body["token"] = auth.Token
	case "mcp_oauth":
		body["access_token"] = auth.AccessToken
		auth.ExpiresAt.apply(body, "expires_at")
		if auth.Refresh != nil {
			body["refresh"] = oauthRefreshMap(*auth.Refresh)
		}
	}
	return body
}

func credentialRotateMap(rotate CredentialRotate) map[string]any {
	body := map[string]any{
		"type": rotate.AuthType,
	}
	rotate.Token.apply(body, "token")
	rotate.AccessToken.apply(body, "access_token")
	rotate.ExpiresAt.apply(body, "expires_at")
	if rotate.Refresh.Present {
		if rotate.Refresh.Null {
			body["refresh"] = nil
		} else {
			body["refresh"] = oauthRefreshMap(rotate.Refresh.Value)
		}
	}
	return body
}

func oauthRefreshMap(refresh OAuthRefreshWrite) map[string]any {
	body := map[string]any{
		"token_endpoint": refresh.TokenEndpoint,
		"client_id":      refresh.ClientID,
	}
	if refresh.RefreshToken != "" {
		body["refresh_token"] = refresh.RefreshToken
	}
	if refresh.TokenEndpointAuth != nil {
		auth := map[string]any{"type": refresh.TokenEndpointAuth.Type}
		if refresh.TokenEndpointAuth.ClientSecret != "" {
			auth["client_secret"] = refresh.TokenEndpointAuth.ClientSecret
		}
		body["token_endpoint_auth"] = auth
	}
	return body
}
