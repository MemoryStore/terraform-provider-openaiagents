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

func TestAccAgentResourceAPIDefaults(t *testing.T) {
	fake := testFake
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
					func(s *terraform.State) error {
						id := s.RootModule().Resources["openaiagents_agent.test"].Primary.ID
						return assertAgentCreateSendsDefaults(fake.AgentLastWrite(id))
					},
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

func assertAgentCreateSendsDefaults(body []byte) error {
	if len(body) == 0 {
		return fmt.Errorf("missing agent create payload")
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("decode agent create payload: %w", err)
	}
	if payload["service_tier"] != "auto" {
		return fmt.Errorf("create payload service_tier = %#v, want auto", payload["service_tier"])
	}
	ma, ok := payload["multi_agent"].(map[string]any)
	if !ok {
		return fmt.Errorf("create payload missing multi_agent: %#v", payload["multi_agent"])
	}
	if jsonInt(ma["max_concurrent_subagents"]) != 6 {
		return fmt.Errorf("create payload max_concurrent_subagents = %#v, want 6", ma["max_concurrent_subagents"])
	}
	tools, ok := payload["tools"].([]any)
	if !ok || len(tools) < 4 {
		return fmt.Errorf("create payload tools = %#v", payload["tools"])
	}
	fn, _ := tools[0].(map[string]any)
	if fn["defer_loading"] != false {
		return fmt.Errorf("create payload function.defer_loading = %#v, want false", fn["defer_loading"])
	}
	ptc, _ := tools[1].(map[string]any)
	if ptc["enabled"] != true {
		return fmt.Errorf("create payload programmatic_tool_calling.enabled = %#v, want true", ptc["enabled"])
	}
	ws, _ := tools[2].(map[string]any)
	if ws["type"] != "web_search" {
		return fmt.Errorf("create payload tools[2].type = %#v, want web_search", ws["type"])
	}
	mcp, _ := tools[3].(map[string]any)
	if mcp["required"] != false {
		return fmt.Errorf("create payload mcp.required = %#v, want false", mcp["required"])
	}
	return nil
}

func jsonInt(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	default:
		return 0
	}
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

func TestAccAgentResourceStdioRequiresCommand(t *testing.T) {
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
      server_label = "local"
      transport = {
        type = "stdio"
        cwd  = "/workspace"
      }
    }
  }]
}
`,
				ExpectError: regexp.MustCompile("command and cwd"),
			},
		},
	})
}

func TestAccAgentResourceStdioRequiresCwd(t *testing.T) {
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
      server_label = "local"
      transport = {
        type    = "stdio"
        command = "python"
      }
    }
  }]
}
`,
				ExpectError: regexp.MustCompile("command and cwd"),
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
