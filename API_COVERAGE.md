# API-to-schema coverage

Reference checked: OpenAI Agents API public beta, 2026-09-11.

- Guides: https://developers.openai.com/api/docs/guides/agents-api/overview
- Create agent: https://developers.openai.com/api/reference/resources/beta/subresources/agents/methods/create
- Templates: https://developers.openai.com/api/reference/resources/beta/subresources/agents/subresources/environments/subresources/templates/methods/create
- Vaults: https://developers.openai.com/api/docs/guides/agents-api/tools/vaults
- Header: `OpenAI-Beta: agents=v1`
- SDK: official `openai-go` v3.61.0 includes Agents endpoints. This provider uses a typed HTTP adapter so omit/null/empty JSON and tests stay explicit. The SDK is not a runtime dependency.

This provider implements persistent deployment objects only. Sessions, turns, messages, and executors are out of scope.

## `openaiagents_agent` / data source `openaiagents_agent`

| API field | Terraform | Create | Read | Update | Notes |
| --- | --- | --- | --- | --- | --- |
| `model` | `model` | required | yes | yes | Stored as supplied |
| `instructions` | `instructions` | optional | yes | null clears | Omit preserves on update |
| `name` | `name` | optional | yes | null clears | |
| `metadata` | `metadata` | optional | yes | empty map clears | Default empty map |
| `reasoning` | `reasoning` | optional | only if configured | null clears | API-resolved defaults are not copied into omitted blocks |
| `service_tier` | `service_tier` | optional | only if configured | null clears | API default `auto` when omitted |
| `text` | `text` (`schema_json` for JSON Schema) | optional | only if configured | null clears | |
| `multi_agent` | `multi_agent` | optional | only if configured | null clears | Removing the block disables multi-agent |
| `tools[]` | `tools` | optional | yes | list replaces | Default empty list |
| `id` | `id` | computed | yes | identity | |
| `created_at` / `updated_at` | `created_at` / `updated_at` | computed | yes | | Unix seconds |
| `object` | `object` | computed | yes | | |

### Tool union

| Variant | Status | Tested |
| --- | --- | --- |
| `function` | implemented (`parameters_json`) | yes, including `additionalProperties: false` |
| `tool_search` | implemented | yes |
| `programmatic_tool_calling` | implemented | yes, including `enabled: false` |
| `mcp` (HTTP/stdio, credential-free) | implemented | yes, including omitted vs empty `allowed_tools` |
| `web_search` | implemented | yes |
| inline `transport.authorization` | rejected | yes |
| credential-bearing headers | rejected | yes |
| other tool types | rejected, never dropped | yes |

MCP preserves URL/transport, server label, allowed tools, connection origin, non-secret headers, credential reference, request metadata, and required initialization.

## `openaiagents_environment_template`

| API field | Terraform | Readback | Notes |
| --- | --- | --- | --- |
| `name` | `name` | yes | |
| `capability_directories` | `capability_directories` | yes | |
| `network` | `network` | yes | |
| `packages` | `packages` | yes | |
| `files` metadata | `files` path/type/`file_id` | metadata only | Inline `data` is write-only |
| `files[].data` | `files[].data` write-only | **not read back** | Sent when `files_revision` changes |
| `skills` references | `skills` | metadata | Pin `version` for reproducible deploys |
| `skills` archive bytes | `skills[].source_data` write-only | **not read back** | `skills_revision` |
| `plugins` archive bytes | `plugins[].source_data` write-only | **not read back** | `plugins_revision` |
| `env` values | `env` write-only | **not read back** | `env_revision`; `env_keys` if the API returns names |
| `setup_commands[].command` | `setup_commands[].command` write-only | **not read back** | `setup_commands_revision`; `cwd` is observable |

Creating a template does not create a session. Complete drift detection is not claimed for confidential inputs.

## `openaiagents_vault`

| API field | Terraform | Notes |
| --- | --- | --- |
| `name` | `name` | Replacement-only; no vault update API |
| `metadata` | `metadata` | Replacement-only |
| `id` / `created_at` | computed | |

Delete removes the vault and its credentials remotely. The provider does not enumerate children or cancel sessions.

## `openaiagents_vault_credential`

| API field | Terraform | Notes |
| --- | --- | --- |
| `vault_id` | `vault_id` | Requires replace. Import `vault_id/credential_id` |
| `name` | `name` | Requires replace |
| `auth.type` | `auth_type` | `static_bearer`, `mcp_oauth`. Requires replace |
| `auth.mcp_server_url` | `mcp_server_url` | Requires replace |
| `auth.token` | `token` write-only | Rotate with `token_revision` |
| `auth.access_token` | `access_token` write-only | OAuth |
| `auth.expires_at` | `expires_at` | Null on rotation clears stored expiry |
| `auth.refresh` | `refresh` | Stores obtained grant material only |
| `auth.refresh.scope` | `refresh.scope` | Sent on create/update; not interactive OAuth |
| `auth.refresh.resource` | `refresh.resource` | Sent on create/update; not interactive OAuth |
| refresh/client secrets | write-only | Never in state |

`expires_at`, `refresh.scope`, and `refresh.resource` are sent on update even when `token_revision` is unchanged. Secret rotation still requires incrementing `token_revision` together with the replacement secret. A true no-op apply (no secret, expiry, or refresh change) does not call the API.

A no-op apply does not rotate. Changing revision without the replacement secret fails with a diagnostic.

## Deferred / out of scope

- Paginated list data sources and name-based discovery
- Native Skills API upload/version resources
- Standalone plugin resource
- Sessions, turns, messages, streaming
- `openai/openai` admin resources and `OPENAI_ADMIN_KEY`
- OpenTofu compatibility claims
