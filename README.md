# Terraform Provider for the OpenAI Agents API

Manage **hosted OpenAI Agents API** deployment objects with Terraform: saved agents, hosted environment templates, and vault credentials.

This is **not** the Assistants API, Responses API, Agents SDK, or Azure agent services. Plan and apply never create sessions, send messages, or run turns.

Provider source: [`MemoryStore/openaiagents`](https://registry.terraform.io/providers/MemoryStore/openaiagents)  
Resource prefix: `openaiagents_*`  
Terraform: `>= 1.11` (write-only arguments)

Published on the Terraform Registry as [`MemoryStore/openaiagents`](https://registry.terraform.io/providers/MemoryStore/openaiagents) (`v0.1.2`).

## Resources and data sources

| Resource | Data source | Remote object |
| --- | --- | --- |
| `openaiagents_agent` | `openaiagents_agent` | `POST/GET/POST/DELETE /v1/agents` |
| `openaiagents_environment_template` | `openaiagents_environment_template` | `/v1/agents/environments/templates` |
| `openaiagents_vault` | `openaiagents_vault` | `/v1/vaults` (no update API) |
| `openaiagents_vault_credential` | `openaiagents_vault_credential` | `/v1/vaults/{id}/credentials` |

Field-level coverage, rejected tool variants, and template readback limits: [API_COVERAGE.md](API_COVERAGE.md).

## Authentication

Configure a **project API key**:

- Provider argument `api_key` (sensitive), or
- Environment variable `OPENAI_API_KEY`

Optional: `organization` (`OPENAI_ORG_ID` / `OPENAI_ORGANIZATION`), `project` (`OPENAI_PROJECT`), `base_url` (`OPENAI_BASE_URL`, default `https://api.openai.com/v1`).

This provider **never reads or forwards `OPENAI_ADMIN_KEY`**. Use `openai/openai` separately for organization administration.

Requests send `OpenAI-Beta: agents=v1`.

## Local development

```shell
go install
```

Development override (optional; the Registry package is `MemoryStore/openaiagents`):

```hcl
provider_installation {
  dev_overrides {
    "MemoryStore/openaiagents" = "/home/YOU/go/bin"
  }
  direct {}
}
```

Then `terraform plan` / `apply` against a configuration that sets `OPENAI_API_KEY` in the environment.

```shell
make test      # offline unit tests and fake-API Terraform lifecycle tests (sets TF_ACC=1, never contacts api.openai.com)
make generate  # docs from schema
```

Ordinary tests and CI never contact `api.openai.com`. Live acceptance is opt-in (`OPENAIAGENTS_ACC_LIVE=1`) and is not the default `make test` path.

## Secrets

Bearer tokens, OAuth tokens, client secrets, environment values, setup-command bodies, and archive bytes are write-only. They are not stored in state. Use non-secret revision attributes (`token_revision`, `env_revision`, `files_revision`, …) to rotate or re-upload.

See `examples/resources/openaiagents_vault_credential` for an ephemeral token input.

## Immutable releases

Saved agents and templates are mutable remote objects. For an immutable release, key configuration by an external artifact digest, create new objects, publish the new IDs, and retain old objects while sessions still need them. `create_before_destroy` alone does not keep old IDs after apply.

The `examples/consumer` module exports `agent_id`, optional `environment_template_id`, `vault_ids`, and `artifact_digest` without Memory Store internals.

## License

MPL-2.0. Scaffolding originated from HashiCorp's Terraform Plugin Framework template.
