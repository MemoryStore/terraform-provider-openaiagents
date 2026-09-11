// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccAgentResourceAPIDefaults(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_agent" "test" {
  model = "gpt-6-astra"
  multi_agent = {
    enabled = true
  }
  tools = [
    {
      type = "function"
      function = {
        name        = "lookup"
        description = "Look up"
        parameters_json = jsonencode({ type = "object" })
      }
    },
    {
      type = "programmatic_tool_calling"
    },
    {
      type = "web_search"
    },
    {
      type = "mcp"
      mcp = {
        server_label = "docs"
        transport = {
          type       = "http"
          server_url = "https://developers.openai.com/mcp"
        }
      }
    }
  ]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("openaiagents_agent.test", "service_tier", "auto"),
					resource.TestCheckResourceAttr("openaiagents_agent.test", "multi_agent.max_concurrent_subagents", "6"),
					resource.TestCheckResourceAttr("openaiagents_agent.test", "tools.0.function.defer_loading", "false"),
					resource.TestCheckResourceAttr("openaiagents_agent.test", "tools.1.programmatic_tool_calling.enabled", "true"),
					resource.TestCheckResourceAttr("openaiagents_agent.test", "tools.3.mcp.required", "false"),
				),
			},
			{
				Config: testConfig() + `
resource "openaiagents_agent" "test" {
  model = "gpt-6-astra"
  multi_agent = {
    enabled = true
  }
  tools = [
    {
      type = "function"
      function = {
        name        = "lookup"
        description = "Look up"
        parameters_json = jsonencode({ type = "object" })
      }
    },
    {
      type = "programmatic_tool_calling"
    },
    {
      type = "web_search"
    },
    {
      type = "mcp"
      mcp = {
        server_label = "docs"
        transport = {
          type       = "http"
          server_url = "https://developers.openai.com/mcp"
        }
      }
    }
  ]
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccAgentResourceNullToolRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_agent" "test" {
  model = "gpt-6-astra"
  tools = [null]
}
`,
				ExpectError: regexp.MustCompile("must not be null"),
			},
		},
	})
}

func TestAccAgentResourceSecretHeadersRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_agent" "test" {
  model = "gpt-6-astra"
  tools = [{
    type = "mcp"
    mcp = {
      server_label = "docs"
      transport = {
        type       = "http"
        server_url = "https://example.com/mcp"
        headers = {
          "X-Api-Token" = "secret"
        }
      }
    }
  }]
}
`,
				ExpectError: regexp.MustCompile("credential-bearing"),
			},
		},
	})
}

func TestAccAgentResourceHTTPTransportRequiresURL(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_agent" "test" {
  model = "gpt-6-astra"
  tools = [{
    type = "mcp"
    mcp = {
      server_label = "docs"
      transport = {
        type = "http"
      }
    }
  }]
}
`,
				ExpectError: regexp.MustCompile("server_url"),
			},
		},
	})
}

func TestAccAgentResourceExternalNameDrift(t *testing.T) {
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
  name  = "original"
}
`,
				Check: func(s *terraform.State) error {
					id = s.RootModule().Resources["openaiagents_agent.test"].Primary.ID
					return nil
				},
			},
			{
				PreConfig: func() {
					fake.SetAgentName(id, "externally-changed")
				},
				Config: testConfig() + `
resource "openaiagents_agent" "test" {
  model = "gpt-6-astra"
  name  = "original"
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectNonEmptyPlan()},
				},
			},
		},
	})
}
