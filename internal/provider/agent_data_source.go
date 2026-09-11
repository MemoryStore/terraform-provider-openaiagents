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

var _ datasource.DataSource = &AgentDataSource{}

// AgentDataSource looks up a saved agent by ID.
type AgentDataSource struct {
	client *client.Client
}

func NewAgentDataSource() datasource.DataSource { return &AgentDataSource{} }

func (d *AgentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent"
}

func (d *AgentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up a saved Agents API agent by ID. A missing agent is an error.",
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Required: true, MarkdownDescription: "Remote agent ID."},
			"object":       schema.StringAttribute{Computed: true, MarkdownDescription: "Object type."},
			"created_at":   schema.Int64Attribute{Computed: true, MarkdownDescription: "Unix create timestamp."},
			"updated_at":   schema.Int64Attribute{Computed: true, MarkdownDescription: "Unix update timestamp."},
			"model":        schema.StringAttribute{Computed: true, MarkdownDescription: "Model name."},
			"instructions": schema.StringAttribute{Computed: true, MarkdownDescription: "Custom instructions."},
			"name":         schema.StringAttribute{Computed: true, MarkdownDescription: "Agent name."},
			"metadata":     schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Metadata map."},
			"service_tier": schema.StringAttribute{Computed: true, MarkdownDescription: "Resolved service tier."},
		},
	}
}

type agentDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Object       types.String `tfsdk:"object"`
	CreatedAt    types.Int64  `tfsdk:"created_at"`
	UpdatedAt    types.Int64  `tfsdk:"updated_at"`
	Model        types.String `tfsdk:"model"`
	Instructions types.String `tfsdk:"instructions"`
	Name         types.String `tfsdk:"name"`
	Metadata     types.Map    `tfsdk:"metadata"`
	ServiceTier  types.String `tfsdk:"service_tier"`
}

func (d *AgentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (d *AgentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data agentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	agent, err := d.client.GetAgent(ctx, data.ID.ValueString())
	if err != nil {
		addClientError(&resp.Diagnostics, "Unable to read agent", err)
		return
	}
	data.Object = types.StringValue(agent.Object)
	data.CreatedAt = types.Int64Value(agent.CreatedAt)
	data.UpdatedAt = types.Int64Value(agent.UpdatedAt)
	data.Model = types.StringValue(agent.Model)
	data.Instructions = stringPtrValue(agent.Instructions)
	data.Name = stringPtrValue(agent.Name)
	data.ServiceTier = types.StringValue(agent.ServiceTier)
	meta := agent.Metadata
	if meta == nil {
		meta = map[string]string{}
	}
	mv, diags := types.MapValueFrom(ctx, types.StringType, meta)
	resp.Diagnostics.Append(diags...)
	data.Metadata = mv
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
