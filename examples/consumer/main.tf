terraform {
  required_version = ">= 1.11"

  required_providers {
    openaiagents = {
      source = "MemoryStore/openaiagents"
    }
  }
}

variable "releases" {
  description = "Retained OpenAI deployments keyed by compiler release ID. Non-secret metadata only. Removing a key destroys that release's remote objects."
  type = map(object({
    model        = string
    name         = optional(string)
    instructions = optional(string)
    metadata     = optional(map(string))
    service_tier = optional(string)
    reasoning = optional(object({
      effort  = optional(string)
      summary = optional(string)
    }))
    text = optional(object({
      verbosity = optional(string)
      format = optional(object({
        type        = string
        schema_json = optional(string)
      }))
    }))
    multi_agent = optional(object({
      enabled                  = bool
      max_concurrent_subagents = optional(number)
    }))
    tools = optional(list(object({
      type = string
      function = optional(object({
        name            = string
        description     = string
        parameters_json = string
        defer_loading   = optional(bool)
      }))
      programmatic_tool_calling = optional(object({
        enabled = optional(bool)
      }))
      mcp = optional(object({
        server_label          = string
        allowed_tools         = optional(list(string))
        connection_origin     = optional(string)
        credential_id         = optional(string)
        request_metadata_json = optional(string)
        required              = optional(bool)
        transport = object({
          type       = string
          server_url = optional(string)
          headers    = optional(map(string))
          command    = optional(string)
          cwd        = optional(string)
          args       = optional(list(string))
          env_vars   = optional(list(string))
        })
      }))
      web_search = optional(object({
        allowed_domains = optional(list(string))
        context_size    = optional(string)
        mode            = optional(string)
        location = optional(object({
          city     = optional(string)
          country  = optional(string)
          region   = optional(string)
          timezone = optional(string)
        }))
      }))
    })), [])
    environment_template = optional(object({
      name                   = optional(string)
      capability_directories = optional(list(string))
      network = optional(object({
        access          = optional(string)
        allowed_domains = optional(list(string))
      }))
      packages = optional(object({
        python = optional(list(string))
        system = optional(list(string))
        npm    = optional(list(string))
      }))
      files = optional(list(object({
        type    = string
        path    = string
        file_id = optional(string)
      })))
      files_revision = optional(string)
      skills = optional(list(object({
        type              = string
        name              = optional(string)
        description       = optional(string)
        skill_id          = optional(string)
        version           = optional(string)
        source_media_type = optional(string)
      })))
      skills_revision = optional(string)
      plugins = optional(list(object({
        type              = string
        name              = optional(string)
        description       = optional(string)
        source_media_type = optional(string)
      })))
      plugins_revision = optional(string)
      env_revision     = optional(string)
      setup_commands = optional(list(object({
        cwd = optional(string)
      })))
      setup_commands_revision = optional(string)
    }))
  }))
}

variable "confidential" {
  description = "Write-only values keyed by the same release ID as var.releases. Never put these in ordinary variables or saved plans. Pass via an ephemeral variable, TF_VAR_confidential, or a gitignored -var-file."
  type = map(object({
    env                  = optional(map(string))
    setup_command_bodies = optional(list(string))
    file_data            = optional(list(string))
    skill_data           = optional(list(string))
    plugin_data          = optional(list(string))
  }))
  ephemeral = true
  sensitive = true
  default   = {}
}

variable "active_release" {
  type        = string
  description = "Release key currently bound by the application. Rollback is changing this to a retained key. Cleanup is removing a key from releases."
}

variable "vault_ids" {
  type    = list(string)
  default = []
}

resource "openaiagents_agent" "release" {
  for_each = var.releases

  model        = each.value.model
  name         = each.value.name
  instructions = each.value.instructions
  metadata     = each.value.metadata
  service_tier = each.value.service_tier
  reasoning    = each.value.reasoning
  text         = each.value.text
  multi_agent  = each.value.multi_agent
  tools        = each.value.tools
}

resource "openaiagents_environment_template" "release" {
  for_each = {
    for k, v in var.releases : k => v.environment_template
    if v.environment_template != null
  }

  name                   = each.value.name
  capability_directories = each.value.capability_directories
  network                = each.value.network
  packages               = each.value.packages
  files = [
    for i, f in coalesce(each.value.files, []) : {
      type    = f.type
      path    = f.path
      file_id = f.file_id
      data    = try(var.confidential[each.key].file_data[i], null)
    }
  ]
  files_revision = each.value.files_revision
  skills = [
    for i, s in coalesce(each.value.skills, []) : {
      type              = s.type
      name              = s.name
      description       = s.description
      skill_id          = s.skill_id
      version           = s.version
      source_media_type = s.source_media_type
      source_data       = try(var.confidential[each.key].skill_data[i], null)
    }
  ]
  skills_revision = each.value.skills_revision
  plugins = [
    for i, p in coalesce(each.value.plugins, []) : {
      type              = p.type
      name              = p.name
      description       = p.description
      source_media_type = p.source_media_type
      source_data       = try(var.confidential[each.key].plugin_data[i], null)
    }
  ]
  plugins_revision = each.value.plugins_revision
  env              = try(var.confidential[each.key].env, null)
  env_revision     = each.value.env_revision
  setup_commands = [
    for i, c in coalesce(each.value.setup_commands, []) : {
      cwd     = c.cwd
      command = try(var.confidential[each.key].setup_command_bodies[i], null)
    }
  ]
  setup_commands_revision = each.value.setup_commands_revision
}

output "agent_id" {
  value = openaiagents_agent.release[var.active_release].id
}

output "environment_template_id" {
  value = try(openaiagents_environment_template.release[var.active_release].id, null)
}

output "vault_ids" {
  value = var.vault_ids
}

output "releases" {
  description = "Remote IDs owned by each retained release. CMA can store these as native OpenAI agent versions."
  value = {
    for k, agent in openaiagents_agent.release : k => {
      agent_id                = agent.id
      environment_template_id = try(openaiagents_environment_template.release[k].id, null)
    }
  }
}

output "deployment_binding" {
  value = {
    backend                 = "openai_agents"
    active_release          = var.active_release
    agent_id                = openaiagents_agent.release[var.active_release].id
    environment_template_id = try(openaiagents_environment_template.release[var.active_release].id, null)
    vault_ids               = var.vault_ids
  }
}
