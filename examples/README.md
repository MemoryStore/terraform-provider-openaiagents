# Examples

These configurations target the hosted OpenAI Agents API. They do not create sessions or run turns.

Set `OPENAI_API_KEY` in the environment. Do not check keys into this repository. This provider never reads `OPENAI_ADMIN_KEY`.

Terraform 1.11 or later is required for write-only arguments.

The Registry source is [`MemoryStore/openaiagents`](https://registry.terraform.io/providers/MemoryStore/openaiagents). For local builds, a development override is optional:

```hcl
provider_installation {
  dev_overrides {
    "MemoryStore/openaiagents" = "/path/to/go/bin"
  }
  direct {}
}
```
