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

var _ datasource.DataSource = &EnvironmentTemplateDataSource{}

// EnvironmentTemplateDataSource looks up a template by ID.
type EnvironmentTemplateDataSource struct {
	client *client.Client
}

func NewEnvironmentTemplateDataSource() datasource.DataSource {
	return &EnvironmentTemplateDataSource{}
}

func (d *EnvironmentTemplateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_template"
}

func (d *EnvironmentTemplateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an environment template by ID. Confidential fields are not returned.",
		Attributes: map[string]schema.Attribute{
			"id":                     schema.StringAttribute{Required: true, MarkdownDescription: "Remote template ID."},
			"object":                 schema.StringAttribute{Computed: true, MarkdownDescription: "Object type."},
			"name":                   schema.StringAttribute{Computed: true, MarkdownDescription: "Display name."},
			"created_at":             schema.Int64Attribute{Computed: true, MarkdownDescription: "Unix create timestamp."},
			"updated_at":             schema.Int64Attribute{Computed: true, MarkdownDescription: "Unix update timestamp."},
			"capability_directories": schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Capability directories."},
			"env_keys":               schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Environment variable names, if the API returns them. Values are never returned."},
		},
	}
}

type templateDataSourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Object                types.String `tfsdk:"object"`
	Name                  types.String `tfsdk:"name"`
	CreatedAt             types.Int64  `tfsdk:"created_at"`
	UpdatedAt             types.Int64  `tfsdk:"updated_at"`
	CapabilityDirectories types.List   `tfsdk:"capability_directories"`
	EnvKeys               types.List   `tfsdk:"env_keys"`
}

func (d *EnvironmentTemplateDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (d *EnvironmentTemplateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data templateDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tpl, err := d.client.GetTemplate(ctx, data.ID.ValueString())
	if err != nil {
		addClientError(&resp.Diagnostics, "Unable to read environment template", err)
		return
	}
	data.Object = types.StringValue(tpl.Object)
	data.Name = stringPtrValue(tpl.Name)
	data.CreatedAt = types.Int64Value(tpl.CreatedAt)
	data.UpdatedAt = types.Int64Value(tpl.UpdatedAt)
	dirs, diags := types.ListValueFrom(ctx, types.StringType, tpl.CapabilityDirectories)
	resp.Diagnostics.Append(diags...)
	data.CapabilityDirectories = dirs
	keys, diags := types.ListValueFrom(ctx, types.StringType, tpl.EnvKeys)
	resp.Diagnostics.Append(diags...)
	data.EnvKeys = keys
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
