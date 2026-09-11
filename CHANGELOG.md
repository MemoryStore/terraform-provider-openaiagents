## Unreleased

BUG FIXES:

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
