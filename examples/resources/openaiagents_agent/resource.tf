resource "openaiagents_agent" "minimal" {
  model        = "gpt-6-astra"
  name         = "support"
  instructions = "Answer clearly and concisely."
}
