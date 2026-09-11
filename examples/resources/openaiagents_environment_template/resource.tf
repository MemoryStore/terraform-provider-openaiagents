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
      source_data       = var.plugin_archive_base64
      source_media_type = "application/zip"
    }
  ]
  plugins_revision = var.plugin_archive_revision
}
