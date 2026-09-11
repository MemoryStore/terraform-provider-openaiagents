# Compiler-shaped input. Tools use the Terraform nested schema
# (function = { name, description, parameters_json }), not flat Agents API JSON.
# files_revision/skills_revision/plugins_revision are content digests of
# non-secret bundle metadata. env_revision and setup_commands_revision are
# opaque tokens, not hashes of secrets.
#
# Confidential write-only values belong in an ephemeral `confidential`
# variable (TF_VAR_confidential or a gitignored -var-file), not here.

active_release = "2026-09-11.1"

releases = {
  "2026-09-11.1" = {
    model        = "gpt-6-astra"
    name         = "support"
    instructions = "Answer clearly and concisely."
    metadata = {
      env = "prod"
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
          parameters_json = "{\"additionalProperties\":false,\"properties\":{\"ticket_id\":{\"type\":\"string\"}},\"required\":[\"ticket_id\"],\"type\":\"object\"}"
        }
      },
      {
        type = "web_search"
      }
    ]
    environment_template = {
      name = "support-env"
      files = [
        {
          type = "inline"
          path = "/workspace/hello.txt"
        }
      ]
      files_revision = "sha256:nonsecret-files-2026-09-11.1"
      setup_commands = [{
        cwd = "/workspace"
      }]
      setup_commands_revision = "setup-v1"
    }
  }
}
