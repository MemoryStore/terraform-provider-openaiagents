// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCredentialNeedsRemoteUpdateIgnoresWriteOnlySecrets(t *testing.T) {
	refresh := types.ObjectValueMust(refreshAttrTypes, map[string]attr.Value{
		"token_endpoint": types.StringValue("https://auth.example.com/token"),
		"client_id":      types.StringValue("client"),
		"refresh_token":  types.StringNull(),
		"scope":          types.StringValue("mcp:read"),
		"resource":       types.StringNull(),
		"token_endpoint_auth": types.ObjectValueMust(tokenEndpointAuthAttrTypes, map[string]attr.Value{
			"type":          types.StringValue("client_secret_post"),
			"client_secret": types.StringNull(),
		}),
	})
	plan := vaultCredentialModel{
		AuthType:      types.StringValue("mcp_oauth"),
		ExpiresAt:     types.StringValue("2026-12-01T00:00:00Z"),
		TokenRevision: types.Int64Value(1),
		Refresh:       refresh,
	}
	state := plan
	if credentialNeedsRemoteUpdate(plan, state) {
		t.Fatal("matching non-secret refresh fields should not rotate")
	}

	planWithSecret := plan
	planWithSecret.Refresh = types.ObjectValueMust(refreshAttrTypes, map[string]attr.Value{
		"token_endpoint": types.StringValue("https://auth.example.com/token"),
		"client_id":      types.StringValue("client"),
		"refresh_token":  types.StringUnknown(),
		"scope":          types.StringValue("mcp:read"),
		"resource":       types.StringNull(),
		"token_endpoint_auth": types.ObjectValueMust(tokenEndpointAuthAttrTypes, map[string]attr.Value{
			"type":          types.StringValue("client_secret_post"),
			"client_secret": types.StringUnknown(),
		}),
	})
	if credentialNeedsRemoteUpdate(planWithSecret, state) {
		t.Fatal("write-only refresh secrets must not trigger rotation")
	}

	planScope := plan
	planScope.Refresh = types.ObjectValueMust(refreshAttrTypes, map[string]attr.Value{
		"token_endpoint": types.StringValue("https://auth.example.com/token"),
		"client_id":      types.StringValue("client"),
		"refresh_token":  types.StringNull(),
		"scope":          types.StringValue("mcp:write"),
		"resource":       types.StringNull(),
		"token_endpoint_auth": types.ObjectValueMust(tokenEndpointAuthAttrTypes, map[string]attr.Value{
			"type":          types.StringValue("client_secret_post"),
			"client_secret": types.StringNull(),
		}),
	})
	if !credentialNeedsRemoteUpdate(planScope, state) {
		t.Fatal("scope change should rotate")
	}
}

func TestCredentialNeedsRemoteUpdateStaticBearerIgnoresExpiresAt(t *testing.T) {
	plan := vaultCredentialModel{
		AuthType:      types.StringValue("static_bearer"),
		ExpiresAt:     types.StringValue("2027-06-06T00:00:00Z"),
		TokenRevision: types.Int64Value(1),
	}
	state := vaultCredentialModel{
		AuthType:      types.StringValue("static_bearer"),
		ExpiresAt:     types.StringNull(),
		TokenRevision: types.Int64Value(1),
	}
	if credentialNeedsRemoteUpdate(plan, state) {
		t.Fatal("static_bearer expires_at must not rotate")
	}
	plan.TokenRevision = types.Int64Value(2)
	if !credentialNeedsRemoteUpdate(plan, state) {
		t.Fatal("token_revision change should rotate")
	}
}
