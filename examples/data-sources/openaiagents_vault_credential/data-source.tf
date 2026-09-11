data "openaiagents_vault_credential" "selected" {
  vault_id = openaiagents_vault.shared.id
  id       = openaiagents_vault_credential.docs.id
}
