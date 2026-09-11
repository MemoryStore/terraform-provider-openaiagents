// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/MemoryStore/terraform-provider-openaiagents/internal/client"
)

func configureClient(providerData any, diags *diag.Diagnostics) *client.Client {
	if providerData == nil {
		return nil
	}
	c, ok := providerData.(*client.Client)
	if !ok {
		diags.AddError(
			"Unexpected provider data",
			fmt.Sprintf("Expected *client.Client, got: %T", providerData),
		)
		return nil
	}
	return c
}

func addClientError(diags *diag.Diagnostics, summary string, err error) {
	diags.AddError(summary, err.Error())
}

func validateImportID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("import id is empty")
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, "?#") {
		return fmt.Errorf("malformed import id")
	}
	return nil
}

func parseVaultCredentialImportID(id string) (vaultID, credentialID string, err error) {
	if err := validateImportID(id); err != nil {
		return "", "", err
	}
	parts := strings.Split(id, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("vault credential import id must be vault_id/credential_id")
	}
	return parts[0], parts[1], nil
}
