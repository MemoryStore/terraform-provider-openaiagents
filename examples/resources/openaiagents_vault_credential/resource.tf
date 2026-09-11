ephemeral "terraform_data" "mcp_token" {
  input = var.mcp_bearer_token
}

resource "openaiagents_vault_credential" "docs" {
  vault_id       = openaiagents_vault.shared.id
  name           = "docs-mcp"
  auth_type      = "static_bearer"
  mcp_server_url = "https://mcp.example.com"
  token          = ephemeral.terraform_data.mcp_token.input
  token_revision = var.mcp_token_revision
}
