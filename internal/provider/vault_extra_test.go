// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccVaultCredentialExpiresAtConverges(t *testing.T) {
	fake := testFake
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_vault" "test" { name = "v" }
resource "openaiagents_vault_credential" "test" {
  vault_id       = openaiagents_vault.test.id
  auth_type      = "mcp_oauth"
  mcp_server_url = "https://mcp.example.com"
  access_token   = "tok"
  expires_at     = "2026-12-01T00:00:00Z"
  token_revision = 1
}
`,
			},
			{
				Config: testConfig() + `
resource "openaiagents_vault" "test" { name = "v" }
resource "openaiagents_vault_credential" "test" {
  vault_id       = openaiagents_vault.test.id
  auth_type      = "mcp_oauth"
  mcp_server_url = "https://mcp.example.com"
  access_token   = "tok"
  expires_at     = "2027-06-06T00:00:00Z"
  token_revision = 1
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("openaiagents_vault_credential.test", "expires_at", "2027-06-06T00:00:00Z"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources["openaiagents_vault_credential.test"].Primary.ID
						if fake.CredentialRotateCount(id) < 1 {
							return fmt.Errorf("expected a remote rotation for expires_at, got %d", fake.CredentialRotateCount(id))
						}
						return nil
					},
				),
			},
			{
				Config: testConfig() + `
resource "openaiagents_vault" "test" { name = "v" }
resource "openaiagents_vault_credential" "test" {
  vault_id       = openaiagents_vault.test.id
  auth_type      = "mcp_oauth"
  mcp_server_url = "https://mcp.example.com"
  access_token   = "tok"
  expires_at     = "2027-06-06T00:00:00Z"
  token_revision = 1
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccVaultCredentialOAuthScopeResourceUpdate(t *testing.T) {
	fake := testFake
	var rotates int
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_vault" "test" { name = "v" }
resource "openaiagents_vault_credential" "test" {
  vault_id       = openaiagents_vault.test.id
  auth_type      = "mcp_oauth"
  mcp_server_url = "https://mcp.example.com"
  access_token   = "tok"
  expires_at     = "2026-12-01T00:00:00Z"
  token_revision = 1
  refresh = {
    token_endpoint = "https://auth.example.com/token"
    client_id      = "client"
    refresh_token  = "rt"
    scope          = "mcp:read"
    resource       = "https://mcp.example.com"
  }
}
`,
				Check: func(s *terraform.State) error {
					id := s.RootModule().Resources["openaiagents_vault_credential.test"].Primary.ID
					got := fake.StoredRefresh(id)
					if got.Scope != "mcp:read" || got.Resource != "https://mcp.example.com" {
						return fmt.Errorf("stored refresh = %+v", got)
					}
					return nil
				},
			},
			{
				Config: testConfig() + `
resource "openaiagents_vault" "test" { name = "v" }
resource "openaiagents_vault_credential" "test" {
  vault_id       = openaiagents_vault.test.id
  auth_type      = "mcp_oauth"
  mcp_server_url = "https://mcp.example.com"
  access_token   = "tok"
  expires_at     = "2026-12-01T00:00:00Z"
  token_revision = 1
  refresh = {
    token_endpoint = "https://auth.example.com/token"
    client_id      = "client"
    refresh_token  = "rt"
    scope          = "mcp:read mcp:write"
    resource       = "https://mcp.example.com/v2"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("openaiagents_vault_credential.test", "refresh.scope", "mcp:read mcp:write"),
					resource.TestCheckResourceAttr("openaiagents_vault_credential.test", "refresh.resource", "https://mcp.example.com/v2"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources["openaiagents_vault_credential.test"].Primary.ID
						rotates = fake.CredentialRotateCount(id)
						if rotates < 1 {
							return fmt.Errorf("expected a remote rotation for scope/resource, got %d", rotates)
						}
						got := fake.StoredRefresh(id)
						if got.Scope != "mcp:read mcp:write" {
							return fmt.Errorf("stored scope = %q", got.Scope)
						}
						if got.Resource != "https://mcp.example.com/v2" {
							return fmt.Errorf("stored resource = %q", got.Resource)
						}
						var auth map[string]any
						if err := json.Unmarshal(fake.CredentialLastAuth(id), &auth); err != nil {
							return fmt.Errorf("decode last auth: %w", err)
						}
						refresh, _ := auth["refresh"].(map[string]any)
						if refresh["scope"] != "mcp:read mcp:write" {
							return fmt.Errorf("rotate payload scope = %#v", refresh["scope"])
						}
						if refresh["resource"] != "https://mcp.example.com/v2" {
							return fmt.Errorf("rotate payload resource = %#v", refresh["resource"])
						}
						return nil
					},
				),
			},
			{
				Config: testConfig() + `
resource "openaiagents_vault" "test" { name = "v" }
resource "openaiagents_vault_credential" "test" {
  vault_id       = openaiagents_vault.test.id
  auth_type      = "mcp_oauth"
  mcp_server_url = "https://mcp.example.com"
  access_token   = "tok"
  expires_at     = "2026-12-01T00:00:00Z"
  token_revision = 1
  refresh = {
    token_endpoint = "https://auth.example.com/token"
    client_id      = "client"
    refresh_token  = "rt"
    scope          = "mcp:read mcp:write"
    resource       = "https://mcp.example.com/v2"
  }
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: func(s *terraform.State) error {
					id := s.RootModule().Resources["openaiagents_vault_credential.test"].Primary.ID
					if fake.CredentialRotateCount(id) != rotates {
						return fmt.Errorf("no-op apply rotated credentials")
					}
					return nil
				},
			},
		},
	})
}

func TestAccVaultCredentialStaticBearerExpiresAtRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_vault" "test" { name = "v" }
resource "openaiagents_vault_credential" "test" {
  vault_id       = openaiagents_vault.test.id
  auth_type      = "static_bearer"
  mcp_server_url = "https://mcp.example.com"
  token          = "t"
  expires_at     = "2027-06-06T00:00:00Z"
}
`,
				ExpectError: regexp.MustCompile("expires_at is only valid for mcp_oauth"),
			},
		},
	})
}

func TestAccVaultCredentialDeleteAfterVaultGone(t *testing.T) {
	fake := testFake
	var vaultID string
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_vault" "test" { name = "v" }
resource "openaiagents_vault_credential" "test" {
  vault_id       = openaiagents_vault.test.id
  auth_type      = "static_bearer"
  mcp_server_url = "https://mcp.example.com"
  token          = "t"
}
`,
				Check: func(s *terraform.State) error {
					vaultID = s.RootModule().Resources["openaiagents_vault.test"].Primary.ID
					return nil
				},
			},
			{
				PreConfig: func() {
					fake.DeleteVault(vaultID)
				},
				Config: testConfig(),
			},
		},
	})
}
