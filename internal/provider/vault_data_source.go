// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/MemoryStore/terraform-provider-openaiagents/internal/client"
)

var _ datasource.DataSource = &VaultDataSource{}

// VaultDataSource looks up a vault by ID.
type VaultDataSource struct {
	client *client.Client
}

func NewVaultDataSource() datasource.DataSource { return &VaultDataSource{} }

func (d *VaultDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vault"
}

func (d *VaultDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up a vault by ID. Secrets are never returned.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Required: true, MarkdownDescription: "Remote vault ID."},
			"object":     schema.StringAttribute{Computed: true, MarkdownDescription: "Object type."},
			"created_at": schema.Int64Attribute{Computed: true, MarkdownDescription: "Unix create timestamp."},
			"name":       schema.StringAttribute{Computed: true, MarkdownDescription: "Vault name."},
			"metadata":   schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Metadata map."},
		},
	}
}

func (d *VaultDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (d *VaultDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data vaultModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	vault, err := d.client.GetVault(ctx, data.ID.ValueString())
	if err != nil {
		addClientError(&resp.Diagnostics, "Unable to read vault", err)
		return
	}
	state, diags := vaultToState(ctx, data, vault)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
