terraform {
  required_version = ">= 1.11"

  required_providers {
    openaiagents = {
      source = "MemoryStore/openaiagents"
    }
  }
}

provider "openaiagents" {}

variable "model" {
  type    = string
  default = "gpt-6-astra"
}

variable "instructions" {
  type        = string
  description = "Rendered instructions from an external authoring system."
}

variable "name" {
  type    = string
  default = null
}

variable "tools_json" {
  type        = string
  default     = "[]"
  description = "JSON array of persisted Agents API tool objects produced by the authoring compiler."
}

variable "environment_template_id" {
  type    = string
  default = null
}

variable "vault_ids" {
  type    = list(string)
  default = []
}

variable "artifact_digest" {
  type        = string
  description = "Non-secret digest of the authored artifacts. Not an OpenAI revision."
}

locals {
  tools = jsondecode(var.tools_json)
}

resource "openaiagents_agent" "this" {
  model        = var.model
  name         = var.name
  instructions = var.instructions
  tools        = local.tools
}

output "agent_id" {
  value = openaiagents_agent.this.id
}

output "environment_template_id" {
  value = var.environment_template_id
}

output "vault_ids" {
  value = var.vault_ids
}

output "deployment_binding" {
  value = {
    backend                 = "openai_agents"
    agent_id                = openaiagents_agent.this.id
    environment_template_id = var.environment_template_id
    vault_ids               = var.vault_ids
    artifact_digest         = var.artifact_digest
  }
}
