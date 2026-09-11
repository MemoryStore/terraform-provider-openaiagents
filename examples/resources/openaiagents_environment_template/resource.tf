resource "openaiagents_environment_template" "skills" {
  name = "report-env"
  capability_directories = [
    "/workspace/capabilities/skills"
  ]

  network = {
    access = "restricted"
    allowed_domains = [
      "api.example.com"
    ]
  }

  packages = {
    python = ["pandas==2.2.3"]
  }

  skills = [
    {
      type     = "skill_reference"
      skill_id = "skill_abc"
      version  = "1"
    }
  ]

  plugins = [
    {
      type              = "inline"
      name              = "docs-helper"
      description       = "Find answers in documentation."
      source_data       = "UEsDBAoAAAAAAAEAAA=="
      source_media_type = "application/zip"
    }
  ]
  plugins_revision = "1"
}
