// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
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
