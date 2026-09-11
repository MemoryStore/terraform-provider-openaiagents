// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package client_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/MemoryStore/terraform-provider-openaiagents/internal/client"
	"github.com/MemoryStore/terraform-provider-openaiagents/internal/testfake"
)

func TestCreateAgentSendsBetaHeaderAndNotAdminKey(t *testing.T) {
	t.Setenv("OPENAI_ADMIN_KEY", "ADMIN_CANARY_DO_NOT_SEND")
	fake := testfake.Start(t)
	c, err := client.New(client.Options{
		APIKey:          "sk-test",
		BaseURL:         fake.URL(),
		AllowProduction: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := c.CreateAgent(context.Background(), client.AgentWrite{Model: "gpt-6-astra"})
	if err != nil {
		t.Fatal(err)
	}
	if agent.ID == "" {
		t.Fatal("expected remote id")
	}
	req := fake.LastRequest()
	if req.Headers.Get("OpenAI-Beta") != "agents=v1" {
		t.Fatalf("missing beta header: %v", req.Headers)
	}
	if got := req.Headers.Get("Authorization"); got != "Bearer sk-test" {
		t.Fatalf("authorization = %q", got)
	}
	for _, values := range req.Headers {
		for _, v := range values {
			if strings.Contains(v, "ADMIN_CANARY_DO_NOT_SEND") {
				t.Fatal("admin key leaked into request headers")
			}
		}
	}
}

func TestAgentUpdateSendsNullToClear(t *testing.T) {
	fake := testfake.Start(t)
	c, err := client.New(client.Options{APIKey: "sk-test", BaseURL: fake.URL()})
	if err != nil {
		t.Fatal(err)
	}
	created, err := c.CreateAgent(context.Background(), client.AgentWrite{
		Model:        "gpt-6-astra",
		Instructions: client.Set("hello"),
		Name:         client.Set("n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.UpdateAgent(context.Background(), created.ID, client.AgentWrite{
		Model:        "gpt-6-astra",
		Instructions: client.Null[string](),
		Name:         client.Null[string](),
	})
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(fake.LastRequest().Body, &body); err != nil {
		t.Fatal(err)
	}
	if body["instructions"] != nil {
		t.Fatalf("expected instructions null, got %#v", body["instructions"])
	}
	if body["name"] != nil {
		t.Fatalf("expected name null, got %#v", body["name"])
	}
}

func TestValidatePersistedToolsRejectsUnknownAndSecrets(t *testing.T) {
	err := client.ValidatePersistedTools([]json.RawMessage{[]byte(`{"type":"computer"}`)})
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
	err = client.ValidatePersistedTools([]json.RawMessage{[]byte(`{"type":"mcp","transport":{"type":"http","server_url":"https://x","authorization":"Bearer x"}}`)})
	if err == nil || !strings.Contains(err.Error(), "authorization") {
		t.Fatalf("expected authorization error, got %v", err)
	}
	err = client.ValidatePersistedTools([]json.RawMessage{[]byte(`{"type":"mcp","transport":{"type":"stdio","cwd":"/workspace"}}`)})
	if err == nil || !strings.Contains(err.Error(), "command and cwd") {
		t.Fatalf("expected stdio command/cwd error, got %v", err)
	}
	err = client.ValidatePersistedTools([]json.RawMessage{[]byte(`{"type":"mcp","transport":{"type":"stdio","command":"python"}}`)})
	if err == nil || !strings.Contains(err.Error(), "command and cwd") {
		t.Fatalf("expected stdio command/cwd error, got %v", err)
	}
}

func TestRejectExpandedSecretHeaders(t *testing.T) {
	for _, h := range []string{"X-Api-Token", "Api-Key", "X-Goog-Api-Key"} {
		err := client.ValidatePersistedTools([]json.RawMessage{[]byte(`{"type":"mcp","server_label":"x","transport":{"type":"http","server_url":"https://x","headers":{"` + h + `":"secret"}}}`)})
		if err == nil {
			t.Fatalf("expected rejection for header %s", h)
		}
	}
}

func TestCanonicalJSONPreservesFalseBoolean(t *testing.T) {
	got, err := client.CanonicalJSON(`{"type":"object","additionalProperties":false,"properties":{"id":{"type":"string"}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"additionalProperties":false`) {
		t.Fatalf("lost additionalProperties false: %s", got)
	}
}

func TestRefuseProductionWithoutAllow(t *testing.T) {
	c, err := client.New(client.Options{
		APIKey:          "sk-test",
		BaseURL:         "https://api.openai.com/v1",
		AllowProduction: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GetAgent(context.Background(), "agent_x")
	if err == nil || !strings.Contains(err.Error(), "production") {
		t.Fatalf("expected production refusal, got %v", err)
	}
}

func TestBaseURLRejectsHTTPNonLoopback(t *testing.T) {
	_, err := client.New(client.Options{APIKey: "sk-test", BaseURL: "http://example.com/v1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRedirectCrossOriginRejected(t *testing.T) {
	evil := listenAndServe(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	origin := listenAndServe(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil+"/v1/agents", http.StatusFound)
	}))
	c, err := client.New(client.Options{APIKey: "sk-test", BaseURL: origin + "/v1"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GetAgent(context.Background(), "agent_x")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "redirect") {
		t.Fatalf("expected redirect error, got %v", err)
	}
}

func listenAndServe(t *testing.T, h http.Handler) string {
	t.Helper()
	var lastErr error
	for attempt := 0; attempt < 30; attempt++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			lastErr = err
			continue
		}
		srv := &http.Server{Handler: h}
		go func() { _ = srv.Serve(ln) }()
		addr := ln.Addr().String()
		deadline := time.Now().Add(250 * time.Millisecond)
		for time.Now().Before(deadline) {
			c, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
			if err == nil {
				_ = c.Close()
				t.Cleanup(func() { _ = srv.Close() })
				return "http://" + addr
			}
			lastErr = err
			time.Sleep(10 * time.Millisecond)
		}
		_ = srv.Close()
	}
	t.Fatalf("listenAndServe: %v", lastErr)
	return ""
}

func TestMalformedIDRejected(t *testing.T) {
	fake := testfake.Start(t)
	c, err := client.New(client.Options{APIKey: "sk-test", BaseURL: fake.URL()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GetAgent(context.Background(), "../evil")
	if err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("expected malformed id error, got %v", err)
	}
}

func TestDeleteAgentIdempotent(t *testing.T) {
	fake := testfake.Start(t)
	c, err := client.New(client.Options{APIKey: "sk-test", BaseURL: fake.URL()})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteAgent(context.Background(), "agent_missing"); err != nil {
		t.Fatal(err)
	}
}

func TestCredentialNestedAuthRoundTrip(t *testing.T) {
	fake := testfake.Start(t)
	c, err := client.New(client.Options{APIKey: "sk-test", BaseURL: fake.URL()})
	if err != nil {
		t.Fatal(err)
	}
	vault, err := c.CreateVault(context.Background(), client.VaultWrite{Name: client.Set("v")})
	if err != nil {
		t.Fatal(err)
	}
	created, err := c.CreateCredential(context.Background(), vault.ID, client.CredentialCreate{
		Name: "c",
		Auth: client.CredentialAuthWrite{
			Type:         "static_bearer",
			MCPServerURL: "https://mcp.example.com",
			Token:        "CANARY_SECRET_DO_NOT_LEAK",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.AuthType != "static_bearer" {
		t.Fatalf("create did not lift nested auth.type, got %q", created.AuthType)
	}
	if created.MCPServerURL != "https://mcp.example.com" {
		t.Fatalf("create did not lift nested auth.mcp_server_url, got %q", created.MCPServerURL)
	}

	req, err := http.NewRequest(http.MethodGet, fake.URL()+"/vaults/"+vault.ID+"/credentials/"+created.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer sk-test")
	req.Header.Set("OpenAI-Beta", "agents=v1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "CANARY_SECRET_DO_NOT_LEAK") {
		t.Fatal("secret leaked in nested-auth GET body")
	}
	var wire map[string]any
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	if _, ok := wire["auth_type"]; ok {
		t.Fatalf("fake emitted flattened auth_type: %s", body)
	}
	if _, ok := wire["mcp_server_url"]; ok {
		t.Fatalf("fake emitted flattened mcp_server_url: %s", body)
	}
	auth, _ := wire["auth"].(map[string]any)
	if auth == nil {
		t.Fatalf("fake omitted nested auth: %s", body)
	}
	if auth["type"] != "static_bearer" {
		t.Fatalf("nested auth.type = %#v in %s", auth["type"], body)
	}
	if auth["mcp_server_url"] != "https://mcp.example.com" {
		t.Fatalf("nested auth.mcp_server_url = %#v in %s", auth["mcp_server_url"], body)
	}
	var decoded client.Credential
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.AuthType != "static_bearer" || decoded.MCPServerURL != "https://mcp.example.com" {
		t.Fatalf("UnmarshalJSON did not lift nested auth from fake body: %+v", decoded)
	}

	got, err := c.GetCredential(context.Background(), vault.ID, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.AuthType != "static_bearer" || got.MCPServerURL != "https://mcp.example.com" {
		t.Fatalf("GetCredential did not lift nested auth: %+v", got)
	}
}

func TestCredentialUnmarshalNestedOAuthExpiresAt(t *testing.T) {
	raw := []byte(`{
		"id": "cred_from_api",
		"object": "vault.credential",
		"created_at": 1,
		"updated_at": 2,
		"vault_id": "vault_from_api",
		"name": "oauth",
		"auth": {
			"type": "mcp_oauth",
			"mcp_server_url": "https://mcp.example.com",
			"expires_at": "2026-12-01T00:00:00Z"
		}
	}`)
	var cred client.Credential
	if err := json.Unmarshal(raw, &cred); err != nil {
		t.Fatal(err)
	}
	if cred.ID != "cred_from_api" {
		t.Fatalf("id = %q", cred.ID)
	}
	if cred.AuthType != "mcp_oauth" {
		t.Fatalf("auth.type not lifted: %q", cred.AuthType)
	}
	if cred.MCPServerURL != "https://mcp.example.com" {
		t.Fatalf("auth.mcp_server_url not lifted: %q", cred.MCPServerURL)
	}
	if cred.ExpiresAt == nil || *cred.ExpiresAt != "2026-12-01T00:00:00Z" {
		t.Fatalf("auth.expires_at not lifted: %#v", cred.ExpiresAt)
	}
}

func TestVaultCredentialRotation(t *testing.T) {
	fake := testfake.Start(t)
	c, err := client.New(client.Options{APIKey: "sk-test", BaseURL: fake.URL()})
	if err != nil {
		t.Fatal(err)
	}
	vault, err := c.CreateVault(context.Background(), client.VaultWrite{Name: client.Set("v")})
	if err != nil {
		t.Fatal(err)
	}
	cred, err := c.CreateCredential(context.Background(), vault.ID, client.CredentialCreate{
		Name: "c",
		Auth: client.CredentialAuthWrite{Type: "static_bearer", MCPServerURL: "https://mcp.example.com", Token: "CANARY_SECRET_DO_NOT_LEAK"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.GetCredential(context.Background(), vault.ID, cred.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "CANARY_SECRET_DO_NOT_LEAK") {
		t.Fatal("secret leaked in credential GET")
	}
	if fake.StoredToken(cred.ID) != "CANARY_SECRET_DO_NOT_LEAK" {
		t.Fatal("token not stored internally")
	}
	_, err = c.RotateCredential(context.Background(), vault.ID, cred.ID, client.CredentialRotate{
		AuthType: "static_bearer",
		Token:    client.Set("rotated"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if fake.CredentialRotateCount(cred.ID) != 1 {
		t.Fatalf("rotate count = %d", fake.CredentialRotateCount(cred.ID))
	}
	if fake.StoredToken(cred.ID) != "rotated" {
		t.Fatal("token not rotated")
	}
}

func TestOAuthRefreshScopeResourcePersisted(t *testing.T) {
	fake := testfake.Start(t)
	c, err := client.New(client.Options{APIKey: "sk-test", BaseURL: fake.URL()})
	if err != nil {
		t.Fatal(err)
	}
	vault, err := c.CreateVault(context.Background(), client.VaultWrite{Name: client.Set("v")})
	if err != nil {
		t.Fatal(err)
	}
	cred, err := c.CreateCredential(context.Background(), vault.ID, client.CredentialCreate{
		Name: "c",
		Auth: client.CredentialAuthWrite{
			Type:         "mcp_oauth",
			MCPServerURL: "https://mcp.example.com",
			AccessToken:  "tok",
			ExpiresAt:    client.Set("2026-12-01T00:00:00Z"),
			Refresh: &client.OAuthRefreshWrite{
				TokenEndpoint: "https://auth.example.com/token",
				ClientID:      "client",
				RefreshToken:  "rt",
				Scope:         "mcp:read",
				Resource:      "https://mcp.example.com",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := fake.StoredRefresh(cred.ID)
	if got.Scope != "mcp:read" || got.Resource != "https://mcp.example.com" {
		t.Fatalf("stored refresh after create = %+v", got)
	}
	_, err = c.RotateCredential(context.Background(), vault.ID, cred.ID, client.CredentialRotate{
		AuthType: "mcp_oauth",
		Refresh: client.Set(client.OAuthRefreshWrite{
			TokenEndpoint: "https://auth.example.com/token",
			ClientID:      "client",
			Scope:         "mcp:read mcp:write",
			Resource:      "https://mcp.example.com/v2",
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	got = fake.StoredRefresh(cred.ID)
	if got.Scope != "mcp:read mcp:write" {
		t.Fatalf("stored scope after rotate = %q", got.Scope)
	}
	if got.Resource != "https://mcp.example.com/v2" {
		t.Fatalf("stored resource after rotate = %q", got.Resource)
	}
	var auth map[string]any
	if err := json.Unmarshal(fake.CredentialLastAuth(cred.ID), &auth); err != nil {
		t.Fatal(err)
	}
	refresh, _ := auth["refresh"].(map[string]any)
	if refresh["scope"] != "mcp:read mcp:write" || refresh["resource"] != "https://mcp.example.com/v2" {
		t.Fatalf("rotate auth payload = %#v", auth)
	}
}

func TestTemplateConfidentialNotReadBack(t *testing.T) {
	fake := testfake.Start(t)
	c, err := client.New(client.Options{APIKey: "sk-test", BaseURL: fake.URL()})
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := c.CreateTemplate(context.Background(), client.TemplateWrite{
		SendConfidential: true,
		Name:             client.Set("t"),
		Env:              client.Set(map[string]string{"SECRET": "CANARY_SECRET_DO_NOT_LEAK"}),
		SetupCommands:    client.Set([]client.SetupCommand{{Command: "CANARY_SECRET_DO_NOT_LEAK"}}),
		Files:            client.Set([]client.TemplateFile{{Type: "inline", Path: "/workspace/a.txt", Data: "CANARY_SECRET_DO_NOT_LEAK"}}),
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(tpl)
	if strings.Contains(string(raw), "CANARY_SECRET_DO_NOT_LEAK") {
		t.Fatalf("secret leaked in template create response: %s", raw)
	}
	got, err := c.GetTemplate(context.Background(), tpl.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(got)
	if strings.Contains(string(raw), "CANARY_SECRET_DO_NOT_LEAK") {
		t.Fatalf("secret leaked in template GET: %s", raw)
	}
	if !fake.TemplateHasEnv(tpl.ID, "SECRET", "CANARY_SECRET_DO_NOT_LEAK") {
		t.Fatal("env not stored internally")
	}
}
