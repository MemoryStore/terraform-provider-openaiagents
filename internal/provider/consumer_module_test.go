// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func consumerGeneratedConfig(active string) string {
	return testConfig() + fmt.Sprintf(`
locals {
  active_release = %q
}

resource "openaiagents_agent" "v1" {
  model        = "gpt-6-astra"
  name         = "support"
  instructions = "Answer clearly and concisely."
  tools = [
    {
      type = "function"
      function = {
        name        = "lookup_ticket"
        description = "Look up a ticket by ID"
        parameters_json = jsonencode({
          type                 = "object"
          additionalProperties = false
          properties = {
            ticket_id = { type = "string" }
          }
          required = ["ticket_id"]
        })
      }
    },
    { type = "web_search" }
  ]
}

resource "openaiagents_agent" "v0" {
  model        = "gpt-6-astra"
  name         = "support-previous"
  instructions = "Previous release."
}

resource "openaiagents_environment_template" "v1" {
  name = "support-env"
  files = [{
    type = "inline"
    path = "/workspace/hello.txt"
    data = "aGVsbG8="
  }]
  files_revision          = "sha256:nonsecret-files-2026-09-11.1"
  setup_commands          = [{ command = "mkdir -p /workspace/reports", cwd = "/workspace" }]
  setup_commands_revision = "setup-v1"
}

output "agent_id" {
  value = local.active_release == "2026-09-11.1" ? openaiagents_agent.v1.id : openaiagents_agent.v0.id
}
`, active)
}

func TestAccConsumerGeneratedDeployment(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: consumerGeneratedConfig("2026-09-11.1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("openaiagents_agent.v1", "id"),
					resource.TestCheckResourceAttrSet("openaiagents_agent.v0", "id"),
					resource.TestCheckResourceAttr("openaiagents_agent.v1", "tools.0.function.name", "lookup_ticket"),
					resource.TestCheckResourceAttrSet("openaiagents_environment_template.v1", "id"),
					func(s *terraform.State) error {
						current := s.RootModule().Resources["openaiagents_agent.v1"]
						out := s.RootModule().Outputs["agent_id"]
						if current == nil || out == nil || out.Value != current.Primary.ID {
							return fmt.Errorf("active release did not bind current agent")
						}
						return nil
					},
				),
			},
			{
				Config: consumerGeneratedConfig("2026-09-11.0"),
				Check: func(s *terraform.State) error {
					if s.RootModule().Resources["openaiagents_agent.v1"] == nil {
						return fmt.Errorf("rollback destroyed the previous current release")
					}
					prev := s.RootModule().Resources["openaiagents_agent.v0"]
					out := s.RootModule().Outputs["agent_id"]
					if prev == nil || out == nil || out.Value != prev.Primary.ID {
						return fmt.Errorf("rollback did not select retained agent")
					}
					return nil
				},
			},
		},
	})
}
