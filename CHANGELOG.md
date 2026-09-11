## Unreleased

BUG FIXES:

* MCP `server_url` values supplied by another resource are no longer treated as empty during plan; resolved values are validated at apply

IMPROVEMENTS:

* Consumer module `confidential` is an ephemeral variable for env values, command bodies, and archive bytes; `releases` holds non-secret metadata only
* Consumer module passes `metadata`, `service_tier`, `reasoning`, `text`, and `multi_agent` through to the agent resource
* Consumer module acceptance tests instantiate the module itself

## 0.1.3 (2026-09-11)

BREAKING CHANGES:

* Template `files_revision`, `skills_revision`, `plugins_revision`, `env_revision`, and `setup_commands_revision` are now strings. Use a content digest for non-secret bundles and an opaque token for confidential `env` and setup commands.

IMPROVEMENTS:

* The consumer module takes typed nested tools, a retained `releases` map, and `active_release` instead of flattening Agents API JSON into the Terraform schema
* Unknown function `parameters_json` waits until apply instead of failing validation
* Conflicting tool variant blocks are rejected instead of silently discarded
* Disabled `multi_agent` no longer plans `max_concurrent_subagents = 6`
* Template import reads back observable network, packages, files, skills, and plugins
* Unknown `multi_agent.max_concurrent_subagents` is no longer replaced with 6 during plan
* An API-empty `capability_directories` list is written to state so external clears are detected

## 0.1.2 (2026-09-11)

BUG FIXES:

* Changing observable template fields (`files[].path`, `setup_commands[].cwd`, env keys, skill/plugin metadata) without bumping the group revision now reaches the API when write-only values remain in configuration, or errors naming the revision attribute to increment

## 0.1.1 (2026-09-11)

BUG FIXES:

* Name-only environment template updates no longer send `files`/`skills`/`plugins` without bytes, which wiped remote inline content
* Creating a template with `setup_commands` no longer fails when the API omits that field on read
* Reordering `web_search` and `programmatic_tool_calling` tools no longer copies nested objects across list indexes
* Bare `web_search` tools now plan the API defaults `context_size=medium` and `mode=live`, so a second apply against the hosted API is empty
* OAuth credential no-op applies no longer treat write-only refresh secrets as a remote change
* `static_bearer` credentials reject `expires_at` and `refresh` instead of posting an empty rotation
* Fake Agents API persists OAuth refresh `scope` and `resource` so grant updates can be verified
* Docs examples for `openaiagents_vault_credential` and `openaiagents_environment_template` are self-contained

IMPROVEMENTS:

* Agent default tests assert the create request payload, not only readback state
* `make test` runs lint and generate before the fake-API test suite
* Stdio MCP tools missing `command` or `cwd` are covered by lifecycle tests
* Fake API listen retry no longer races the Serve goroutine against a replaced `http.Server`

## 0.1.0 (2026-09-11)

FEATURES:

* **New Resource:** `openaiagents_agent` — saved hosted Agents API agent
* **New Resource:** `openaiagents_environment_template` — hosted environment template
* **New Resource:** `openaiagents_vault` — MCP credential vault (no update API)
* **New Resource:** `openaiagents_vault_credential` — static bearer and MCP OAuth material
* **New Data Source:** `openaiagents_agent`
* **New Data Source:** `openaiagents_environment_template`
* **New Data Source:** `openaiagents_vault`
* **New Data Source:** `openaiagents_vault_credential`
