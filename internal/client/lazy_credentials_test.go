// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package client_test

import (
	"context"
	"strings"
	"testing"

	"github.com/MemoryStore/terraform-provider-openaiagents/internal/client"
	"github.com/MemoryStore/terraform-provider-openaiagents/internal/testfake"
)

// The client is constructible without credentials so the provider can be
// configured by roots that enable no resources.
func TestNewAcceptsEmptyAPIKey(t *testing.T) {
	if _, err := client.New(client.Options{BaseURL: "http://127.0.0.1:1/v1"}); err != nil {
		t.Fatalf("New with empty api_key: %v", err)
	}
}

func TestRequestWithoutAPIKeyIsRefusedBeforeSend(t *testing.T) {
	fake := testfake.Start(t)
	c, err := client.New(client.Options{BaseURL: fake.URL()})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"CreateAgent", func() error {
			_, err := c.CreateAgent(context.Background(), client.AgentWrite{Model: "gpt-6-astra"})
			return err
		}},
		{"GetAgent", func() error {
			_, err := c.GetAgent(context.Background(), "agent_123")
			return err
		}},
		{"DeleteAgent", func() error {
			return c.DeleteAgent(context.Background(), "agent_123")
		}},
		{"UpdateAgent", func() error {
			_, err := c.UpdateAgent(context.Background(), "agent_123", client.AgentWrite{Model: "gpt-6-astra"})
			return err
		}},
		{"CreateTemplate", func() error {
			_, err := c.CreateTemplate(context.Background(), client.TemplateWrite{})
			return err
		}},
		{"GetTemplate", func() error {
			_, err := c.GetTemplate(context.Background(), "env_123")
			return err
		}},
		{"CreateVault", func() error {
			_, err := c.CreateVault(context.Background(), client.VaultWrite{})
			return err
		}},
		{"GetVault", func() error {
			_, err := c.GetVault(context.Background(), "vault_123")
			return err
		}},
		{"GetCredential", func() error {
			_, err := c.GetCredential(context.Background(), "vault_123", "cred_123")
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatal("expected a missing credential error")
			}
			if !client.IsMissingAPIKey(err) {
				t.Fatalf("IsMissingAPIKey = false for %v", err)
			}
			if !strings.Contains(err.Error(), "Missing API key") {
				t.Fatalf("error text = %q, want the Missing API key wording", err.Error())
			}
			if client.IsNotFound(err) {
				t.Fatal("a missing key must not be reported as a 404")
			}
		})
	}

	if got := fake.RequestCount(); got != 0 {
		t.Fatalf("fake API saw %d requests; the key must be checked before sending", got)
	}
}

// A whitespace-only key is treated as absent, as it was at construction time.
func TestBlankAPIKeyIsTreatedAsMissing(t *testing.T) {
	fake := testfake.Start(t)
	c, err := client.New(client.Options{APIKey: "   ", BaseURL: fake.URL()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateAgent(context.Background(), client.AgentWrite{Model: "gpt-6-astra"}); !client.IsMissingAPIKey(err) {
		t.Fatalf("IsMissingAPIKey = false for %v", err)
	}
}

// Surrounding whitespace is trimmed rather than producing a malformed header.
func TestPaddedAPIKeyIsTrimmedOntoTheWire(t *testing.T) {
	fake := testfake.Start(t)
	c, err := client.New(client.Options{APIKey: "  sk-test\n", BaseURL: fake.URL()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateAgent(context.Background(), client.AgentWrite{Model: "gpt-6-astra"}); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if got := fake.LastRequest().Headers.Get("Authorization"); got != "Bearer sk-test" {
		t.Fatalf("authorization = %q, want the trimmed key", got)
	}
}

// A resolved key still reaches the API unchanged.
func TestResolvedAPIKeyStillAuthorizesRequests(t *testing.T) {
	fake := testfake.Start(t)
	c, err := client.New(client.Options{APIKey: "sk-test", BaseURL: fake.URL()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateAgent(context.Background(), client.AgentWrite{Model: "gpt-6-astra"}); err != nil {
		t.Fatal(err)
	}
	if got := fake.LastRequest().Headers.Get("Authorization"); got != "Bearer sk-test" {
		t.Fatalf("authorization = %q", got)
	}
}
