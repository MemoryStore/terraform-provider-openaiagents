resource "openaiagents_environment_template" "skills" {
  name = "report-env"
}

data "openaiagents_environment_template" "selected" {
  id = openaiagents_environment_template.skills.id
}
