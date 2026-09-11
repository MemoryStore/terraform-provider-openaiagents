// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func consumerModuleSource() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "examples", "consumer"))
}

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func writeConsumerHarness(t *testing.T, dir, active, providerBin string) {
	t.Helper()
	main, err := os.ReadFile(filepath.Join(consumerModuleSource(), "main.tf"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), main, 0o644); err != nil {
		t.Fatal(err)
	}
	provider := fmt.Sprintf(`
provider "openaiagents" {
  api_key  = "sk-test"
  base_url = %q
}
`, testFake.URL())
	if err := os.WriteFile(filepath.Join(dir, "provider.tf"), []byte(provider), 0o644); err != nil {
		t.Fatal(err)
	}
	tfvars := fmt.Sprintf(`
active_release = %q

releases = {
  "2026-09-11.1" = {
    model        = "gpt-6-astra"
    name         = "support"
    instructions = "Answer clearly and concisely."
    metadata = {
      env = "test"
    }
    service_tier = "auto"
    reasoning = {
      effort = "low"
    }
    text = {
      verbosity = "low"
    }
    multi_agent = {
      enabled = true
    }
    tools = [
      {
        type = "function"
        function = {
          name            = "lookup_ticket"
          description     = "Look up a ticket by ID"
          parameters_json = "{\"type\":\"object\",\"additionalProperties\":false,\"properties\":{\"ticket_id\":{\"type\":\"string\"}},\"required\":[\"ticket_id\"]}"
        }
      },
      {
        type = "web_search"
      }
    ]
    environment_template = {
      name = "support-env"
      files = [{
        type = "inline"
        path = "/workspace/hello.txt"
      }]
      files_revision = "sha256:nonsecret-files-2026-09-11.1"
      setup_commands = [{
        cwd = "/workspace"
      }]
      setup_commands_revision = "setup-v1"
    }
  }
  "2026-09-11.0" = {
    model        = "gpt-6-astra"
    name         = "support-previous"
    instructions = "Previous release."
    tools        = []
  }
}

confidential = {
  "2026-09-11.1" = {
    file_data            = ["aGVsbG8="]
    setup_command_bodies = ["mkdir -p /workspace/reports"]
  }
}
`, active)
	if err := os.WriteFile(filepath.Join(dir, "terraform.tfvars"), []byte(tfvars), 0o644); err != nil {
		t.Fatal(err)
	}
	rc := fmt.Sprintf("provider_installation {\n  dev_overrides {\n    \"MemoryStore/openaiagents\" = %q\n  }\n  direct {}\n}\n", filepath.Dir(providerBin))
	if err := os.WriteFile(filepath.Join(dir, "terraformrc"), []byte(rc), 0o644); err != nil {
		t.Fatal(err)
	}
}

func terraformIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("terraform", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"TF_CLI_CONFIG_FILE="+filepath.Join(dir, "terraformrc"),
		"TF_IN_AUTOMATION=1",
		"TF_ACC=",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("terraform %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func TestAccConsumerGeneratedDeployment(t *testing.T) {
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("terraform CLI is required to test the consumer module")
	}
	install := exec.Command("go", "install", ".")
	install.Dir = repoRoot()
	if out, err := install.CombinedOutput(); err != nil {
		t.Fatalf("go install: %v\n%s", err, out)
	}
	gobin, err := exec.Command("go", "env", "GOBIN").Output()
	if err != nil {
		t.Fatal(err)
	}
	binDir := strings.TrimSpace(string(gobin))
	if binDir == "" {
		gopath, err := exec.Command("go", "env", "GOPATH").Output()
		if err != nil {
			t.Fatal(err)
		}
		binDir = filepath.Join(strings.TrimSpace(string(gopath)), "bin")
	}
	providerBin := filepath.Join(binDir, "terraform-provider-openaiagents")
	if _, err := os.Stat(providerBin); err != nil {
		t.Fatalf("provider binary missing: %v", err)
	}

	dir := t.TempDir()
	writeConsumerHarness(t, dir, "2026-09-11.1", providerBin)
	terraformIn(t, dir, "init", "-input=false", "-backend=false")
	terraformIn(t, dir, "apply", "-auto-approve", "-input=false")
	list := terraformIn(t, dir, "state", "list")
	for _, want := range []string{
		`openaiagents_agent.release["2026-09-11.1"]`,
		`openaiagents_agent.release["2026-09-11.0"]`,
		`openaiagents_environment_template.release["2026-09-11.1"]`,
	} {
		if !strings.Contains(list, want) {
			t.Fatalf("state list missing %s\n%s", want, list)
		}
	}
	show := terraformIn(t, dir, "state", "show", `-no-color`, `openaiagents_agent.release["2026-09-11.1"]`)
	for _, want := range []string{
		`lookup_ticket`,
		`effort`,
		`verbosity`,
		`enabled`,
	} {
		if !strings.Contains(show, want) {
			t.Fatalf("agent show missing %q\n%s", want, show)
		}
	}
	id1 := strings.TrimSpace(terraformIn(t, dir, "output", "-raw", "agent_id"))
	if id1 == "" {
		t.Fatal("empty agent_id")
	}

	writeConsumerHarness(t, dir, "2026-09-11.0", providerBin)
	terraformIn(t, dir, "apply", "-auto-approve", "-input=false")
	id0 := strings.TrimSpace(terraformIn(t, dir, "output", "-raw", "agent_id"))
	if id0 == "" || id0 == id1 {
		t.Fatalf("rollback did not switch active agent: before=%s after=%s", id1, id0)
	}
	list = terraformIn(t, dir, "state", "list")
	if !strings.Contains(list, `openaiagents_agent.release["2026-09-11.1"]`) {
		t.Fatalf("rollback destroyed retained release\n%s", list)
	}
	terraformIn(t, dir, "destroy", "-auto-approve", "-input=false")
}
