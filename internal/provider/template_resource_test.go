// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccEnvironmentTemplateResource(t *testing.T) {
	fake := testFake
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_environment_template" "test" {
  name = "hosted"
  capability_directories = ["/workspace/capabilities/skills"]
  network = {
    access = "restricted"
    allowed_domains = ["api.example.com"]
  }
  packages = {
    python = ["pandas==2.2.3"]
  }
  env = {
    TOKEN = "CANARY_SECRET_DO_NOT_LEAK"
  }
  env_revision = 1
  setup_commands = [
    {
      command = "CANARY_SECRET_DO_NOT_LEAK"
      cwd     = "/workspace"
    }
  ]
  setup_commands_revision = 1
  files = [
    {
      type = "inline"
      path = "/workspace/skill.zip"
      data = "CANARY_SECRET_DO_NOT_LEAK"
    }
  ]
  files_revision = 1
  skills = [
    {
      type    = "skill_reference"
      skill_id = "skill_abc"
      version  = "1"
    }
  ]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("openaiagents_environment_template.test", "id"),
					resource.TestCheckNoResourceAttr("openaiagents_environment_template.test", "env.TOKEN"),
					resource.TestCheckResourceAttr("openaiagents_environment_template.test", "files.0.path", "/workspace/skill.zip"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources["openaiagents_environment_template.test"].Primary.ID
						if !fake.TemplateHasEnv(id, "TOKEN", "CANARY_SECRET_DO_NOT_LEAK") {
							return fmt.Errorf("env not sent to API")
						}
						if fake.TemplateSetupCommand(id) != "CANARY_SECRET_DO_NOT_LEAK" {
							return fmt.Errorf("setup command not stored")
						}
						if fake.TemplateFileData(id) != "CANARY_SECRET_DO_NOT_LEAK" {
							return fmt.Errorf("file data not stored")
						}
						return nil
					},
				),
			},
			{
				Config: testConfig() + `
resource "openaiagents_environment_template" "test" {
  name = "hosted"
  capability_directories = ["/workspace/capabilities/skills"]
  network = {
    access = "restricted"
    allowed_domains = ["api.example.com"]
  }
  packages = {
    python = ["pandas==2.2.3"]
  }
  env = {
    TOKEN = "CANARY_SECRET_DO_NOT_LEAK"
  }
  env_revision = 1
  setup_commands = [
    {
      command = "CANARY_SECRET_DO_NOT_LEAK"
      cwd     = "/workspace"
    }
  ]
  setup_commands_revision = 1
  files = [
    {
      type = "inline"
      path = "/workspace/skill.zip"
      data = "CANARY_SECRET_DO_NOT_LEAK"
    }
  ]
  files_revision = 1
  skills = [
    {
      type    = "skill_reference"
      skill_id = "skill_abc"
      version  = "1"
    }
  ]
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: testConfig() + `
resource "openaiagents_environment_template" "test" {
  name = "hosted-updated"
  capability_directories = ["/workspace/capabilities/skills"]
  env = {
    TOKEN = "NEW_CANARY"
  }
  env_revision = 2
  files_revision = 1
  setup_commands_revision = 1
}
`,
				Check: func(s *terraform.State) error {
					id := s.RootModule().Resources["openaiagents_environment_template.test"].Primary.ID
					if !fake.TemplateHasEnv(id, "TOKEN", "NEW_CANARY") {
						return fmt.Errorf("env revision did not update stored value")
					}
					if fake.TemplateSetupCommand(id) != "CANARY_SECRET_DO_NOT_LEAK" {
						return fmt.Errorf("unrelated update erased setup command")
					}
					return nil
				},
			},
		},
	})
}

func TestAccEnvironmentTemplateNameOnlyPreservesFiles(t *testing.T) {
	fake := testFake
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_environment_template" "test" {
  name = "files"
  files = [
    {
      type = "inline"
      path = "/workspace/skill.zip"
      data = "CANARY_SECRET_DO_NOT_LEAK"
    }
  ]
  files_revision = 1
  plugins = [
    {
      type              = "inline"
      name              = "docs-helper"
      description       = "Find answers"
      source_data       = "CANARY_PLUGIN"
      source_media_type = "application/zip"
    }
  ]
  plugins_revision = 1
}
`,
				Check: func(s *terraform.State) error {
					id := s.RootModule().Resources["openaiagents_environment_template.test"].Primary.ID
					if fake.TemplateFileData(id) != "CANARY_SECRET_DO_NOT_LEAK" {
						return fmt.Errorf("file data not stored")
					}
					return nil
				},
			},
			{
				Config: testConfig() + `
resource "openaiagents_environment_template" "test" {
  name = "files-renamed"
  files = [
    {
      type = "inline"
      path = "/workspace/skill.zip"
      data = "CANARY_SECRET_DO_NOT_LEAK"
    }
  ]
  files_revision = 1
  plugins = [
    {
      type              = "inline"
      name              = "docs-helper"
      description       = "Find answers"
      source_data       = "CANARY_PLUGIN"
      source_media_type = "application/zip"
    }
  ]
  plugins_revision = 1
}
`,
				Check: func(s *terraform.State) error {
					id := s.RootModule().Resources["openaiagents_environment_template.test"].Primary.ID
					if fake.TemplateFileData(id) != "CANARY_SECRET_DO_NOT_LEAK" {
						return fmt.Errorf("name-only update erased inline file content")
					}
					var body map[string]any
					if err := json.Unmarshal(fake.TemplateLastWrite(id), &body); err != nil {
						return fmt.Errorf("decode last template write: %w", err)
					}
					if _, ok := body["files"]; ok {
						return fmt.Errorf("name-only update sent files and would wipe remote bytes: %#v", body["files"])
					}
					if _, ok := body["plugins"]; ok {
						return fmt.Errorf("name-only update sent plugins and would wipe remote archives: %#v", body["plugins"])
					}
					return nil
				},
			},
		},
	})
}

func TestAccEnvironmentTemplateSetupCommandsCreate(t *testing.T) {
	fake := testFake
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testConfig() + `
resource "openaiagents_environment_template" "test" {
  name = "setup"
  setup_commands = [
    {
      command = "mkdir -p reports"
      cwd     = "/workspace"
    }
  ]
  setup_commands_revision = 1
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("openaiagents_environment_template.test", "id"),
					resource.TestCheckResourceAttr("openaiagents_environment_template.test", "setup_commands.0.cwd", "/workspace"),
					resource.TestCheckNoResourceAttr("openaiagents_environment_template.test", "setup_commands.0.command"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources["openaiagents_environment_template.test"].Primary.ID
						if fake.TemplateSetupCommand(id) != "mkdir -p reports" {
							return fmt.Errorf("setup command not stored")
						}
						return nil
					},
				),
			},
			{
				Config: testConfig() + `
resource "openaiagents_environment_template" "test" {
  name = "setup"
  setup_commands = [
    {
      command = "mkdir -p reports"
      cwd     = "/workspace"
    }
  ]
  setup_commands_revision = 1
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
