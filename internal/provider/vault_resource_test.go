// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccVaultAndCredential(t *testing.T) {
	fake := testFake
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_vault" "test" {
  name = "shared"
  metadata = {
    team = "platform"
  }
}

resource "openaiagents_vault_credential" "test" {
  vault_id       = openaiagents_vault.test.id
  name           = "mcp"
  auth_type      = "static_bearer"
  mcp_server_url = "https://mcp.example.com"
  token          = "CANARY_SECRET_DO_NOT_LEAK"
  token_revision = 1
}

data "openaiagents_vault_credential" "meta" {
  vault_id = openaiagents_vault.test.id
  id       = openaiagents_vault_credential.test.id
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("openaiagents_vault.test", "id"),
					resource.TestCheckResourceAttrSet("openaiagents_vault_credential.test", "id"),
					resource.TestCheckResourceAttr("openaiagents_vault_credential.test", "auth_type", "static_bearer"),
					resource.TestCheckResourceAttr("openaiagents_vault_credential.test", "mcp_server_url", "https://mcp.example.com"),
					resource.TestCheckResourceAttr("data.openaiagents_vault_credential.meta", "auth_type", "static_bearer"),
					resource.TestCheckResourceAttr("data.openaiagents_vault_credential.meta", "mcp_server_url", "https://mcp.example.com"),
					resource.TestCheckNoResourceAttr("openaiagents_vault_credential.test", "token"),
					resource.TestCheckNoResourceAttr("data.openaiagents_vault_credential.meta", "token"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources["openaiagents_vault_credential.test"].Primary.ID
						if fake.StoredToken(id) != "CANARY_SECRET_DO_NOT_LEAK" {
							return fmt.Errorf("token did not reach the API")
						}
						return nil
					},
				),
			},
			{
				Config: testConfig() + `
resource "openaiagents_vault" "test" {
  name = "shared"
  metadata = {
    team = "platform"
  }
}

resource "openaiagents_vault_credential" "test" {
  vault_id       = openaiagents_vault.test.id
  name           = "mcp"
  auth_type      = "static_bearer"
  mcp_server_url = "https://mcp.example.com"
  token          = "CANARY_SECRET_DO_NOT_LEAK"
  token_revision = 1
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: func(s *terraform.State) error {
					id := s.RootModule().Resources["openaiagents_vault_credential.test"].Primary.ID
					if fake.CredentialRotateCount(id) != 0 {
						return fmt.Errorf("no-op apply rotated credentials")
					}
					return nil
				},
			},
			{
				Config: testConfig() + `
resource "openaiagents_vault" "test" {
  name = "shared"
  metadata = {
    team = "platform"
  }
}

resource "openaiagents_vault_credential" "test" {
  vault_id       = openaiagents_vault.test.id
  name           = "mcp"
  auth_type      = "static_bearer"
  mcp_server_url = "https://mcp.example.com"
  token          = "rotated-token"
  token_revision = 2
}
`,
				Check: func(s *terraform.State) error {
					id := s.RootModule().Resources["openaiagents_vault_credential.test"].Primary.ID
					if fake.CredentialRotateCount(id) != 1 {
						return fmt.Errorf("expected one rotation, got %d", fake.CredentialRotateCount(id))
					}
					if fake.StoredToken(id) != "rotated-token" {
						return fmt.Errorf("token not rotated")
					}
					return nil
				},
			},
			{
				ResourceName:            "openaiagents_vault_credential.test",
				ImportState:             true,
				ImportStateIdFunc:       testVaultCredentialImportID,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"token", "access_token", "token_revision", "refresh"},
			},
		},
	})
}

func testVaultCredentialImportID(s *terraform.State) (string, error) {
	r := s.RootModule().Resources["openaiagents_vault_credential.test"]
	return r.Primary.Attributes["vault_id"] + "/" + r.Primary.ID, nil
}

func TestAccVaultCredentialMalformedImport(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_vault" "test" {
  name = "v"
}
resource "openaiagents_vault_credential" "test" {
  vault_id       = openaiagents_vault.test.id
  auth_type      = "static_bearer"
  mcp_server_url = "https://mcp.example.com"
  token          = "t"
}
`,
			},
			{
				ResourceName:  "openaiagents_vault_credential.test",
				ImportState:   true,
				ImportStateId: "only-one-part",
				ExpectError:   regexp.MustCompile("vault_id/credential_id"),
			},
		},
	})
}

func TestAccVaultCredentialOAuth(t *testing.T) {
	fake := testFake
	oauthConfig := testConfig() + `
resource "openaiagents_vault" "test" {
  name = "oauth"
}
resource "openaiagents_vault_credential" "test" {
  vault_id       = openaiagents_vault.test.id
  auth_type      = "mcp_oauth"
  mcp_server_url = "https://mcp.example.com"
  access_token   = "CANARY_SECRET_DO_NOT_LEAK"
  expires_at     = "2026-12-01T00:00:00Z"
  token_revision = 1
  refresh = {
    token_endpoint = "https://auth.example.com/token"
    client_id      = "client"
    refresh_token  = "CANARY_SECRET_DO_NOT_LEAK"
    token_endpoint_auth = {
      type          = "client_secret_post"
      client_secret = "CANARY_SECRET_DO_NOT_LEAK"
    }
  }
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: oauthConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("openaiagents_vault_credential.test", "auth_type", "mcp_oauth"),
					resource.TestCheckResourceAttr("openaiagents_vault_credential.test", "mcp_server_url", "https://mcp.example.com"),
					resource.TestCheckResourceAttr("openaiagents_vault_credential.test", "expires_at", "2026-12-01T00:00:00Z"),
					resource.TestCheckNoResourceAttr("openaiagents_vault_credential.test", "access_token"),
					resource.TestCheckNoResourceAttr("openaiagents_vault_credential.test", "refresh.refresh_token"),
					resource.TestCheckNoResourceAttr("openaiagents_vault_credential.test", "refresh.token_endpoint_auth.client_secret"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources["openaiagents_vault_credential.test"].Primary.ID
						if fake.CredentialRotateCount(id) != 0 {
							return fmt.Errorf("create rotated credentials")
						}
						return nil
					},
				),
			},
			{
				Config: oauthConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: func(s *terraform.State) error {
					id := s.RootModule().Resources["openaiagents_vault_credential.test"].Primary.ID
					if fake.CredentialRotateCount(id) != 0 {
						return fmt.Errorf("oauth no-op apply rotated credentials")
					}
					return nil
				},
			},
		},
	})
}

func TestAccAdminKeyNotForwarded(t *testing.T) {
	fake := testFake
	t.Setenv("OPENAI_ADMIN_KEY", "ADMIN_CANARY_DO_NOT_SEND")
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_agent" "test" {
  model = "gpt-6-astra"
}
`,
				Check: func(s *terraform.State) error {
					req := fake.LastRequest()
					for _, values := range req.Headers {
						for _, v := range values {
							if v == "ADMIN_CANARY_DO_NOT_SEND" || v == "Bearer ADMIN_CANARY_DO_NOT_SEND" {
								return fmt.Errorf("admin key forwarded")
							}
						}
					}
					if req.Headers.Get("OpenAI-Beta") != "agents=v1" {
						return fmt.Errorf("missing beta header")
					}
					return nil
				},
			},
		},
	})
}
