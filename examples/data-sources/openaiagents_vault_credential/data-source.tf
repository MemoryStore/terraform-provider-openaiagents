resource "openaiagents_vault" "shared" {
  name = "shared-mcp"
}

resource "openaiagents_vault_credential" "docs" {
  vault_id       = openaiagents_vault.shared.id
  name           = "docs-mcp"
  auth_type      = "static_bearer"
  mcp_server_url = "https://mcp.example.com"
  token          = "example-token"
  token_revision = 1
}

data "openaiagents_vault_credential" "selected" {
  vault_id = openaiagents_vault.shared.id
  id       = openaiagents_vault_credential.docs.id
}
