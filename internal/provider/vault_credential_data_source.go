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

var _ datasource.DataSource = &VaultCredentialDataSource{}

// VaultCredentialDataSource looks up credential metadata. It never returns secrets.
type VaultCredentialDataSource struct {
	client *client.Client
}

func NewVaultCredentialDataSource() datasource.DataSource { return &VaultCredentialDataSource{} }

type vaultCredentialDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	VaultID      types.String `tfsdk:"vault_id"`
	Object       types.String `tfsdk:"object"`
	CreatedAt    types.Int64  `tfsdk:"created_at"`
	UpdatedAt    types.Int64  `tfsdk:"updated_at"`
	Name         types.String `tfsdk:"name"`
	AuthType     types.String `tfsdk:"auth_type"`
	MCPServerURL types.String `tfsdk:"mcp_server_url"`
	ExpiresAt    types.String `tfsdk:"expires_at"`
}

func (d *VaultCredentialDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vault_credential"
}

func (d *VaultCredentialDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up vault credential metadata by vault ID and credential ID. Secrets are never returned.",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Required: true, MarkdownDescription: "Remote credential ID."},
			"vault_id":       schema.StringAttribute{Required: true, MarkdownDescription: "Parent vault ID."},
			"object":         schema.StringAttribute{Computed: true, MarkdownDescription: "Object type."},
			"created_at":     schema.Int64Attribute{Computed: true, MarkdownDescription: "Unix create timestamp."},
			"updated_at":     schema.Int64Attribute{Computed: true, MarkdownDescription: "Unix update timestamp."},
			"name":           schema.StringAttribute{Computed: true, MarkdownDescription: "Credential name."},
			"auth_type":      schema.StringAttribute{Computed: true, MarkdownDescription: "Authentication type."},
			"mcp_server_url": schema.StringAttribute{Computed: true, MarkdownDescription: "Bound MCP server URL."},
			"expires_at":     schema.StringAttribute{Computed: true, MarkdownDescription: "OAuth expiry if present. Never a secret value."},
		},
	}
}

func (d *VaultCredentialDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (d *VaultCredentialDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data vaultCredentialDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cred, err := d.client.GetCredential(ctx, data.VaultID.ValueString(), data.ID.ValueString())
	if err != nil {
		addClientError(&resp.Diagnostics, "Unable to read vault credential", err)
		return
	}
	data.Object = types.StringValue(cred.Object)
	data.CreatedAt = types.Int64Value(cred.CreatedAt)
	data.UpdatedAt = types.Int64Value(cred.UpdatedAt)
	data.Name = stringPtrValue(cred.Name)
	data.AuthType = types.StringValue(cred.AuthType)
	data.MCPServerURL = types.StringValue(cred.MCPServerURL)
	data.ExpiresAt = stringPtrValue(cred.ExpiresAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
