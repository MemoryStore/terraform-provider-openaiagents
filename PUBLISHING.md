# Publishing `MemoryStore/openaiagents`

This provider follows the same release path as [`MemoryStore/terraform-provider-anthropic`](https://github.com/MemoryStore/terraform-provider-anthropic).

## GitHub

- Public repository named `terraform-provider-openaiagents` under the `MemoryStore` org.
- GitHub Actions secrets (same GPG key as the Anthropic provider is fine):
  - `GPG_PRIVATE_KEY` — ASCII-armored private key (RSA or DSA, not ECC)
  - `GPG_PASSPHRASE` — key passphrase
- Pushing a tag `vX.Y.Z` runs `.github/workflows/release.yml` (GoReleaser) and creates a GitHub Release with zipped binaries, `SHA256SUMS`, `SHA256SUMS.sig`, and the registry manifest.

## Terraform Registry

1. Sign in at [registry.terraform.io](https://registry.terraform.io/) with a GitHub account that is an **admin** on the MemoryStore org.
2. Authorize the Terraform Registry GitHub App if prompted (repo + webhook scopes).
3. **Publish** → **Provider** → select `MemoryStore/terraform-provider-openaiagents`.
4. Confirm the **MemoryStore** namespace already has the GPG **public** key that matches `GPG_PRIVATE_KEY` (User Settings → Signing Keys). If Anthropic is already published with the same key, this is already done.
5. After the first signed release exists, the listing is `MemoryStore/openaiagents`.

Do not tag a release until the GPG secrets are on this repository, or GoReleaser will fail.
