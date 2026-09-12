// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/MemoryStore/terraform-provider-openaiagents/internal/client"
)

// configureTestProvider runs Configure against a provider configuration whose
// attributes are the supplied raw values. A nil entry is a null attribute.
func configureTestProvider(t *testing.T, attrs map[string]*string) *fwprovider.ConfigureResponse {
	t.Helper()
	ctx := context.Background()
	p := &OpenAIAgentsProvider{version: "test"}

	schemaResp := &fwprovider.SchemaResponse{}
	p.Schema(ctx, fwprovider.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("provider schema: %v", schemaResp.Diagnostics)
	}

	values := map[string]tftypes.Value{}
	for _, name := range []string{"api_key", "organization", "project", "base_url"} {
		if v, ok := attrs[name]; ok && v != nil {
			values[name] = tftypes.NewValue(tftypes.String, *v)
			continue
		}
		values[name] = tftypes.NewValue(tftypes.String, nil)
	}

	resp := &fwprovider.ConfigureResponse{}
	p.Configure(ctx, fwprovider.ConfigureRequest{
		Config: tfsdk.Config{
			Schema: schemaResp.Schema,
			Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), values),
		},
	}, resp)
	return resp
}

func str(v string) *string { return &v }

// authorizationSentBy drives one request through the configured client and
// reports the Authorization header the fake API observed.
func authorizationSentBy(t *testing.T, resp *fwprovider.ConfigureResponse) string {
	t.Helper()
	c, ok := resp.ResourceData.(*client.Client)
	if !ok {
		t.Fatalf("provider stored %T, want *client.Client", resp.ResourceData)
	}
	if _, err := c.CreateAgent(context.Background(), client.AgentWrite{Model: "gpt-6-astra"}); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	return testFake.LastRequest().Headers.Get("Authorization")
}

// Terraform calls Configure whenever a resource references the provider, which
// includes resources held at count = 0. An absent key must not fail that.
func TestConfigureWithoutAPIKeySucceeds(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	resp := configureTestProvider(t, map[string]*string{"base_url": str(testFake.URL())})

	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no error diagnostics, got %v", resp.Diagnostics)
	}
	if count := len(resp.Diagnostics); count != 0 {
		t.Fatalf("expected zero diagnostics, got %d: %v", count, resp.Diagnostics)
	}
	if resp.ResourceData == nil || resp.DataSourceData == nil {
		t.Fatal("expected a client to be stored for resources and data sources")
	}
}

func TestConfigureFallsBackToEnvAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-from-env")

	resp := configureTestProvider(t, map[string]*string{"base_url": str(testFake.URL())})

	if resp.Diagnostics.HasError() {
		t.Fatalf("configure: %v", resp.Diagnostics)
	}
	if got := authorizationSentBy(t, resp); got != "Bearer sk-from-env" {
		t.Fatalf("authorization = %q, want the environment key", got)
	}
}

func TestConfigureAttributeOverridesEnvAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-from-env")

	resp := configureTestProvider(t, map[string]*string{
		"api_key":  str("sk-from-attribute"),
		"base_url": str(testFake.URL()),
	})

	if resp.Diagnostics.HasError() {
		t.Fatalf("configure: %v", resp.Diagnostics)
	}
	if got := authorizationSentBy(t, resp); got != "Bearer sk-from-attribute" {
		t.Fatalf("authorization = %q, want the attribute key", got)
	}
}

// Unknown provider values still defer configuration entirely.
func TestConfigureDefersOnUnknownAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	ctx := context.Background()
	p := &OpenAIAgentsProvider{version: "test"}

	schemaResp := &fwprovider.SchemaResponse{}
	p.Schema(ctx, fwprovider.SchemaRequest{}, schemaResp)

	resp := &fwprovider.ConfigureResponse{}
	p.Configure(ctx, fwprovider.ConfigureRequest{
		Config: tfsdk.Config{
			Schema: schemaResp.Schema,
			Raw: tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), map[string]tftypes.Value{
				"api_key":      tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
				"organization": tftypes.NewValue(tftypes.String, nil),
				"project":      tftypes.NewValue(tftypes.String, nil),
				"base_url":     tftypes.NewValue(tftypes.String, nil),
			}),
		},
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no error diagnostics, got %v", resp.Diagnostics)
	}
	if resp.ResourceData != nil || resp.DataSourceData != nil {
		t.Fatal("expected configuration to be deferred while api_key is unknown")
	}
}

// The client-side failure must reach practitioners with the wording Configure
// used to emit, so log searches for the old message keep working.
func TestAddClientErrorReportsMissingAPIKey(t *testing.T) {
	var diags diag.Diagnostics
	addClientError(&diags, "Unable to create agent", &client.MissingAPIKeyError{})

	if len(diags) != 1 {
		t.Fatalf("expected one diagnostic, got %d: %v", len(diags), diags)
	}
	if got := diags[0].Summary(); got != "Missing API key" {
		t.Fatalf("summary = %q, want %q", got, "Missing API key")
	}
	want := "Set the provider argument api_key or the OPENAI_API_KEY environment variable. This provider does not use OPENAI_ADMIN_KEY."
	if got := diags[0].Detail(); got != want {
		t.Fatalf("detail = %q, want %q", got, want)
	}
}

// An enabled resource without a key fails at the first API call, not earlier.
func TestAccAgentWithoutAPIKeyFailsAtFirstAPICall(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderConfigWithoutKey() + `
resource "openaiagents_agent" "test" {
  model = "gpt-6-astra"
}
`,
				ExpectError: regexp.MustCompile(`Missing API key`),
			},
		},
	})
}

// A data source read without a key fails the same way.
func TestAccAgentDataSourceWithoutAPIKeyFailsAtFirstAPICall(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderConfigWithoutKey() + `
data "openaiagents_agent" "test" {
  id = "agent_missing_key"
}
`,
				ExpectError: regexp.MustCompile(`Missing API key`),
			},
		},
	})
}

// A root that declares the provider but enables nothing plans without a key.
func TestAccProviderWithoutAPIKeyPlansGatedResources(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProviderConfigWithoutKey() + `
resource "openaiagents_agent" "gated" {
  count = 0
  model = "gpt-6-astra"
}
`,
			},
		},
	})
}
