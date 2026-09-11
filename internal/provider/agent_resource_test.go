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

func TestAccAgentResourceLifecycle(t *testing.T) {
	fake := testFake
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_agent" "test" {
  model        = "gpt-6-astra"
  name         = "lifecycle"
  instructions = "Be helpful."
  metadata = {
    env = "test"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("openaiagents_agent.test", "id"),
					resource.TestCheckResourceAttr("openaiagents_agent.test", "name", "lifecycle"),
					resource.TestCheckResourceAttr("openaiagents_agent.test", "instructions", "Be helpful."),
					func(s *terraform.State) error {
						id := s.RootModule().Resources["openaiagents_agent.test"].Primary.ID
						agent, ok := fake.Agent(id)
						if !ok {
							return fmt.Errorf("fake did not store agent %s", id)
						}
						if agent.Name == nil || *agent.Name != "lifecycle" {
							return fmt.Errorf("fake stored unexpected name")
						}
						return nil
					},
				),
			},
			{
				Config: testConfig() + `
resource "openaiagents_agent" "test" {
  model        = "gpt-6-astra"
  name         = "lifecycle"
  instructions = "Be helpful."
  metadata = {
    env = "test"
  }
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "openaiagents_agent.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testConfig() + `
resource "openaiagents_agent" "test" {
  model = "gpt-6-astra"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("openaiagents_agent.test", "name"),
					resource.TestCheckNoResourceAttr("openaiagents_agent.test", "instructions"),
				),
			},
		},
	})
}

func TestAccAgentResourceExternalDelete(t *testing.T) {
	fake := testFake
	var id string
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_agent" "test" {
  model = "gpt-6-astra"
  name  = "gone"
}
`,
				Check: func(s *terraform.State) error {
					id = s.RootModule().Resources["openaiagents_agent.test"].Primary.ID
					return nil
				},
			},
			{
				PreConfig: func() {
					fake.DeleteAgent(id)
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccAgentResourceTools(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_agent" "test" {
  model = "gpt-6-astra"
  tools = [
    {
      type = "function"
      function = {
        name        = "lookup"
        description = "Look up a record"
        parameters_json = jsonencode({
          type                 = "object"
          additionalProperties = false
          properties = {
            id = { type = "string" }
          }
          required = ["id"]
        })
      }
    },
    {
      type = "tool_search"
    },
    {
      type = "programmatic_tool_calling"
      programmatic_tool_calling = {
        enabled = false
      }
    },
    {
      type = "mcp"
      mcp = {
        server_label = "docs"
        required     = true
        connection_origin = "service"
        allowed_tools     = ["search"]
        transport = {
          type       = "http"
          server_url = "https://developers.openai.com/mcp"
          headers = {
            "X-Tenant-ID" = "tenant"
          }
        }
      }
    },
    {
      type = "web_search"
      web_search = {
        mode         = "live"
        context_size = "low"
      }
    }
  ]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("openaiagents_agent.test", "tools.#", "5"),
					resource.TestCheckResourceAttr("openaiagents_agent.test", "tools.0.function.parameters_json", `{"additionalProperties":false,"properties":{"id":{"type":"string"}},"required":["id"],"type":"object"}`),
					resource.TestCheckResourceAttr("openaiagents_agent.test", "tools.3.mcp.allowed_tools.#", "1"),
				),
			},
			{
				Config: testConfig() + `
resource "openaiagents_agent" "test" {
  model = "gpt-6-astra"
  tools = [
    {
      type = "mcp"
      mcp = {
        server_label = "docs"
        allowed_tools = []
        transport = {
          type       = "http"
          server_url = "https://developers.openai.com/mcp"
        }
      }
    }
  ]
}
`,
				Check: resource.TestCheckResourceAttr("openaiagents_agent.test", "tools.0.mcp.allowed_tools.#", "0"),
			},
		},
	})
}

func TestAccAgentResourceRejectsInlineAuth(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_agent" "test" {
  model = "gpt-6-astra"
  tools = [
    {
      type = "mcp"
      mcp = {
        server_label = "docs"
        transport = {
          type       = "http"
          server_url = "https://example.com/mcp"
          headers = {
            Authorization = "Bearer secret"
          }
        }
      }
    }
  ]
}
`,
				ExpectError: regexp.MustCompile("credential-bearing"),
			},
		},
	})
}

func TestAccAgentResourceMalformedImport(t *testing.T) {
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
			},
			{
				ResourceName:  "openaiagents_agent.test",
				ImportState:   true,
				ImportStateId: "../evil",
				ExpectError:   regexp.MustCompile("malformed"),
			},
		},
	})
}

func TestAccAgentDataSourceMissing(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
data "openaiagents_agent" "missing" {
  id = "agent_does_not_exist"
}
`,
				ExpectError: regexp.MustCompile("404|not found|Unable to read agent"),
			},
		},
	})
}
