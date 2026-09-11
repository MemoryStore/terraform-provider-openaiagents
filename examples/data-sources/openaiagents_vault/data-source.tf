resource "openaiagents_vault" "shared" {
  name = "shared-mcp"
}

data "openaiagents_vault" "selected" {
  id = openaiagents_vault.shared.id
}
