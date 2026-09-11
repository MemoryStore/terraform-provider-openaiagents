resource "openaiagents_vault" "shared" {
  name = "shared-mcp"
  metadata = {
    purpose = "deployment"
  }
}
