terraform {
  required_version = ">= 1.11"

  required_providers {
    openaiagents = {
      source = "MemoryStore/openaiagents"
    }
  }
}

provider "openaiagents" {
  # Supply credentials through OPENAI_API_KEY (and optional OPENAI_ORG_ID /
  # OPENAI_PROJECT). Do not check API keys into configuration.
  # This provider never reads OPENAI_ADMIN_KEY.
  #
  # The key is checked at first use, not here, so a root that declares this
  # provider but enables no objects plans without one.
}
