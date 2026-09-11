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
