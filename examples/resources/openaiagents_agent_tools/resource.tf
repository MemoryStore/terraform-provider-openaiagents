resource "openaiagents_agent" "tools" {
  model        = "gpt-6-astra"
  name         = "researcher"
  instructions = "Use MCP docs and the lookup function. Do not invent credentials."

  tools = [
    {
      type = "function"
      function = {
        name        = "lookup_ticket"
        description = "Look up a ticket by ID"
        parameters_json = jsonencode({
          type                 = "object"
          additionalProperties = false
          properties = {
            ticket_id = { type = "string" }
          }
          required = ["ticket_id"]
        })
      }
    },
    {
      type = "mcp"
      mcp = {
        server_label      = "openai_docs"
        connection_origin = "service"
        required          = true
        credential_id     = openaiagents_vault_credential.docs.id
        transport = {
          type       = "http"
          server_url = "https://developers.openai.com/mcp"
        }
      }
    }
  ]
}
