// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/MemoryStore/terraform-provider-openaiagents/internal/client"
)

var _ resource.Resource = &VaultResource{}
var _ resource.ResourceWithImportState = &VaultResource{}

// VaultResource manages a vault. There is no vault update API; name and metadata changes force replacement.
type VaultResource struct {
	client *client.Client
}

func NewVaultResource() resource.Resource { return &VaultResource{} }

type vaultModel struct {
	ID        types.String `tfsdk:"id"`
	Object    types.String `tfsdk:"object"`
	CreatedAt types.Int64  `tfsdk:"created_at"`
	Name      types.String `tfsdk:"name"`
	Metadata  types.Map    `tfsdk:"metadata"`
}

func (r *VaultResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vault"
}

func (r *VaultResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An Agents API vault for MCP credentials. The inspected API has no vault update operation; changing `name` or `metadata` replaces the vault. " +
			"Deleting a vault removes its credentials remotely. This provider does not enumerate or terminate sessions.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true, MarkdownDescription: "Remote vault ID.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"object":     schema.StringAttribute{Computed: true, MarkdownDescription: "Object type."},
			"created_at": schema.Int64Attribute{Computed: true, MarkdownDescription: "Unix create timestamp.", PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Vault name. Changes force replacement because no vault update API is documented.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"metadata": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "String metadata. Changes force replacement because no vault update API is documented.",
				PlanModifiers:       []planmodifier.Map{mapRequiresReplace{}},
			},
		},
	}
}

type mapRequiresReplace struct{}

func (m mapRequiresReplace) Description(_ context.Context) string {
	return "Requires replacement if the value changes."
}
func (m mapRequiresReplace) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}
func (m mapRequiresReplace) PlanModifyMap(_ context.Context, req planmodifier.MapRequest, resp *planmodifier.MapResponse) {
	if req.ConfigValue.IsUnknown() || req.StateValue.IsUnknown() {
		return
	}
	if !req.ConfigValue.Equal(req.StateValue) {
		resp.RequiresReplace = true
	}
}

func (r *VaultResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *VaultResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vaultModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	write := client.VaultWrite{}
	if !plan.Name.IsNull() {
		write.Name = client.Set(plan.Name.ValueString())
	}
	if !plan.Metadata.IsNull() && !plan.Metadata.IsUnknown() {
		m := map[string]string{}
		resp.Diagnostics.Append(plan.Metadata.ElementsAs(ctx, &m, false)...)
		write.Metadata = client.Set(m)
	}
	vault, err := r.client.CreateVault(ctx, write)
	if err != nil {
		addClientError(&resp.Diagnostics, "Unable to create vault", err)
		return
	}
	state, d := vaultToState(ctx, plan, vault)
	resp.Diagnostics.Append(d...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *VaultResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vaultModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	vault, err := r.client.GetVault(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		addClientError(&resp.Diagnostics, "Unable to read vault", err)
		return
	}
	next, d := vaultToState(ctx, state, vault)
	resp.Diagnostics.Append(d...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *VaultResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Vaults cannot be updated", "The Agents API has no vault update operation. Change name or metadata by replacement.")
}

func (r *VaultResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vaultModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteVault(ctx, state.ID.ValueString()); err != nil {
		addClientError(&resp.Diagnostics, "Unable to delete vault", err)
	}
}

func (r *VaultResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if err := validateImportID(req.ID); err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func vaultToState(ctx context.Context, prior vaultModel, vault *client.Vault) (vaultModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := prior
	out.ID = types.StringValue(vault.ID)
	out.Object = types.StringValue(vault.Object)
	out.CreatedAt = types.Int64Value(vault.CreatedAt)
	out.Name = stringPtrValue(vault.Name)
	meta := vault.Metadata
	if meta == nil {
		meta = map[string]string{}
	}
	if prior.Metadata.IsNull() && len(meta) == 0 {
		out.Metadata = types.MapNull(types.StringType)
	} else {
		mv, d := types.MapValueFrom(ctx, types.StringType, meta)
		diags.Append(d...)
		out.Metadata = mv
	}
	return out, diags
}
