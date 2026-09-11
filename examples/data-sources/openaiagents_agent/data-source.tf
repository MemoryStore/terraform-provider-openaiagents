resource "openaiagents_agent" "minimal" {
  model = "gpt-6-astra"
  name  = "support"
}

data "openaiagents_agent" "selected" {
  id = openaiagents_agent.minimal.id
}
