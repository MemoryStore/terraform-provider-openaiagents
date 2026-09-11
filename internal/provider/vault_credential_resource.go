// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/MemoryStore/terraform-provider-openaiagents/internal/client"
)

var _ resource.Resource = &VaultCredentialResource{}
var _ resource.ResourceWithImportState = &VaultCredentialResource{}

// VaultCredentialResource manages a vault credential.
type VaultCredentialResource struct {
	client *client.Client
}

func NewVaultCredentialResource() resource.Resource { return &VaultCredentialResource{} }

type vaultCredentialModel struct {
	ID            types.String `tfsdk:"id"`
	VaultID       types.String `tfsdk:"vault_id"`
	Object        types.String `tfsdk:"object"`
	CreatedAt     types.Int64  `tfsdk:"created_at"`
	UpdatedAt     types.Int64  `tfsdk:"updated_at"`
	Name          types.String `tfsdk:"name"`
	AuthType      types.String `tfsdk:"auth_type"`
	MCPServerURL  types.String `tfsdk:"mcp_server_url"`
	Token         types.String `tfsdk:"token"`
	AccessToken   types.String `tfsdk:"access_token"`
	ExpiresAt     types.String `tfsdk:"expires_at"`
	Refresh       types.Object `tfsdk:"refresh"`
	TokenRevision types.Int64  `tfsdk:"token_revision"`
}

type oauthRefreshModel struct {
	TokenEndpoint     types.String `tfsdk:"token_endpoint"`
	ClientID          types.String `tfsdk:"client_id"`
	RefreshToken      types.String `tfsdk:"refresh_token"`
	Scope             types.String `tfsdk:"scope"`
	Resource          types.String `tfsdk:"resource"`
	TokenEndpointAuth types.Object `tfsdk:"token_endpoint_auth"`
}

type tokenEndpointAuthModel struct {
	Type         types.String `tfsdk:"type"`
	ClientSecret types.String `tfsdk:"client_secret"`
}

var tokenEndpointAuthAttrTypes = map[string]attr.Type{
	"type":          types.StringType,
	"client_secret": types.StringType,
}

var refreshAttrTypes = map[string]attr.Type{
	"token_endpoint":      types.StringType,
	"client_id":           types.StringType,
	"refresh_token":       types.StringType,
	"scope":               types.StringType,
	"resource":            types.StringType,
	"token_endpoint_auth": types.ObjectType{AttrTypes: tokenEndpointAuthAttrTypes},
}

func (r *VaultCredentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vault_credential"
}

func (r *VaultCredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A vault credential for MCP authentication. Secrets are write-only and never stored in state. " +
			"Use `token_revision` to rotate. Changing `vault_id`, `auth_type`, or `mcp_server_url` forces replacement. " +
			"OAuth support stores already-obtained grant material; this provider does not perform interactive authorization.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Remote credential ID.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"vault_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Parent vault ID. Import format is `vault_id/credential_id`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"object":     schema.StringAttribute{Computed: true, MarkdownDescription: "Object type."},
			"created_at": schema.Int64Attribute{Computed: true, MarkdownDescription: "Unix create timestamp.", PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"updated_at": schema.Int64Attribute{Computed: true, MarkdownDescription: "Unix update timestamp."},
			"name":       schema.StringAttribute{Optional: true, MarkdownDescription: "Credential name.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"auth_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "`static_bearer` or `mcp_oauth`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"mcp_server_url": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "MCP server URL this credential is bound to.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"token": schema.StringAttribute{
				Optional:            true,
				WriteOnly:           true,
				Sensitive:           true,
				MarkdownDescription: "Static bearer token. Write-only. Required for `static_bearer`. Change `token_revision` to rotate.",
			},
			"access_token": schema.StringAttribute{
				Optional:            true,
				WriteOnly:           true,
				Sensitive:           true,
				MarkdownDescription: "OAuth access token. Write-only. Required for `mcp_oauth`.",
			},
			"expires_at": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "OAuth access token expiry as RFC 3339. An explicit null on rotation clears stored expiry.",
			},
			"refresh": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "OAuth refresh configuration. The provider stores already-obtained material and does not run an authorization flow.",
				Attributes: map[string]schema.Attribute{
					"token_endpoint": schema.StringAttribute{Required: true, MarkdownDescription: "Token endpoint URL.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
					"client_id":      schema.StringAttribute{Required: true, MarkdownDescription: "OAuth client ID.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
					"refresh_token":  schema.StringAttribute{Optional: true, WriteOnly: true, Sensitive: true, MarkdownDescription: "OAuth refresh token. Write-only."},
					"scope":          schema.StringAttribute{Optional: true, MarkdownDescription: "OAuth scope sent on refresh. Changing this updates the stored grant through the credential update API."},
					"resource":       schema.StringAttribute{Optional: true, MarkdownDescription: "OAuth resource indicator sent on refresh. Changing this updates the stored grant through the credential update API."},
					"token_endpoint_auth": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Token endpoint authentication.",
						Attributes: map[string]schema.Attribute{
							"type":          schema.StringAttribute{Required: true, MarkdownDescription: "`none`, `client_secret_basic`, or `client_secret_post`.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
							"client_secret": schema.StringAttribute{Optional: true, WriteOnly: true, Sensitive: true, MarkdownDescription: "OAuth client secret. Write-only."},
						},
					},
				},
			},
			"token_revision": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Non-secret rotation marker. Increment together with the replacement secret. A no-op apply does not rotate.",
			},
		},
	}
}

func (r *VaultCredentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *VaultCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vaultCredentialModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var config vaultCredentialModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	auth, d := credentialAuthFromConfig(ctx, plan, config)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	cred, err := r.client.CreateCredential(ctx, plan.VaultID.ValueString(), client.CredentialCreate{
		Name: plan.Name.ValueString(),
		Auth: auth,
	})
	if err != nil {
		addClientError(&resp.Diagnostics, "Unable to create vault credential", err)
		return
	}
	state := credentialToState(plan, cred)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *VaultCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vaultCredentialModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cred, err := r.client.GetCredential(ctx, state.VaultID.ValueString(), state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		addClientError(&resp.Diagnostics, "Unable to read vault credential", err)
		return
	}
	next := credentialToState(state, cred)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *VaultCredentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state vaultCredentialModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	var config vaultCredentialModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !credentialNeedsRemoteUpdate(plan, state) {
		next := credentialToState(plan, &client.Credential{
			ID:           state.ID.ValueString(),
			Object:       state.Object.ValueString(),
			CreatedAt:    state.CreatedAt.ValueInt64(),
			UpdatedAt:    state.UpdatedAt.ValueInt64(),
			VaultID:      state.VaultID.ValueString(),
			Name:         stringPtr(plan.Name),
			AuthType:     plan.AuthType.ValueString(),
			MCPServerURL: plan.MCPServerURL.ValueString(),
			ExpiresAt:    stringPtr(plan.ExpiresAt),
		})
		next.Refresh = plan.Refresh
		resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
		return
	}
	rotate, d := credentialRotateFromConfig(ctx, plan, config, state)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	cred, err := r.client.RotateCredential(ctx, state.VaultID.ValueString(), state.ID.ValueString(), rotate)
	if err != nil {
		addClientError(&resp.Diagnostics, "Unable to rotate vault credential", err)
		return
	}
	next := credentialToState(plan, cred)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *VaultCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vaultCredentialModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteCredential(ctx, state.VaultID.ValueString(), state.ID.ValueString()); err != nil {
		addClientError(&resp.Diagnostics, "Unable to delete vault credential", err)
	}
}

func (r *VaultCredentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	vaultID, credID, err := parseVaultCredentialImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("vault_id"), vaultID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), credID)...)
}

func credentialNeedsRemoteUpdate(plan, state vaultCredentialModel) bool {
	if revisionChanged(plan.TokenRevision, state.TokenRevision) {
		return true
	}
	if !plan.ExpiresAt.Equal(state.ExpiresAt) {
		return true
	}
	if !plan.Refresh.Equal(state.Refresh) {
		return true
	}
	return false
}

func credentialAuthFromConfig(ctx context.Context, plan, config vaultCredentialModel) (client.CredentialAuthWrite, diag.Diagnostics) {
	var diags diag.Diagnostics
	auth := client.CredentialAuthWrite{
		Type:         plan.AuthType.ValueString(),
		MCPServerURL: plan.MCPServerURL.ValueString(),
	}
	switch auth.Type {
	case "static_bearer":
		if config.Token.IsNull() || config.Token.ValueString() == "" {
			diags.AddError("Missing token", "token is required for static_bearer credentials")
			return auth, diags
		}
		auth.Token = config.Token.ValueString()
	case "mcp_oauth":
		if config.AccessToken.IsNull() || config.AccessToken.ValueString() == "" {
			diags.AddError("Missing access_token", "access_token is required for mcp_oauth credentials")
			return auth, diags
		}
		auth.AccessToken = config.AccessToken.ValueString()
		if !plan.ExpiresAt.IsNull() {
			auth.ExpiresAt = client.Set(plan.ExpiresAt.ValueString())
		}
		if !config.Refresh.IsNull() && !config.Refresh.IsUnknown() {
			refresh, d := refreshFromConfig(ctx, config.Refresh)
			diags.Append(d...)
			auth.Refresh = &refresh
		}
	default:
		diags.AddError("Unsupported auth_type", fmt.Sprintf("unsupported auth_type %q; supported: static_bearer, mcp_oauth", auth.Type))
	}
	return auth, diags
}

func credentialRotateFromConfig(ctx context.Context, plan, config, state vaultCredentialModel) (client.CredentialRotate, diag.Diagnostics) {
	var diags diag.Diagnostics
	rotate := client.CredentialRotate{AuthType: plan.AuthType.ValueString()}
	secretRotated := revisionChanged(plan.TokenRevision, state.TokenRevision)
	switch plan.AuthType.ValueString() {
	case "static_bearer":
		if secretRotated {
			if config.Token.IsNull() || config.Token.ValueString() == "" {
				diags.AddError("Missing token", "token_revision changed but token was not provided in configuration")
				return rotate, diags
			}
			rotate.Token = client.Set(config.Token.ValueString())
		}
	case "mcp_oauth":
		if secretRotated {
			if config.AccessToken.IsNull() || config.AccessToken.ValueString() == "" {
				diags.AddError("Missing access_token", "token_revision changed but access_token was not provided in configuration")
				return rotate, diags
			}
			rotate.AccessToken = client.Set(config.AccessToken.ValueString())
		} else if !config.AccessToken.IsNull() && config.AccessToken.ValueString() != "" {
			rotate.AccessToken = client.Set(config.AccessToken.ValueString())
		}
		if plan.ExpiresAt.IsNull() {
			rotate.ExpiresAt = client.Null[string]()
		} else {
			rotate.ExpiresAt = client.Set(plan.ExpiresAt.ValueString())
		}
		if !config.Refresh.IsNull() && !config.Refresh.IsUnknown() {
			refresh, d := refreshFromConfig(ctx, config.Refresh)
			diags.Append(d...)
			rotate.Refresh = client.Set(refresh)
		} else if !plan.Refresh.IsNull() && !plan.Refresh.IsUnknown() {
			refresh, d := refreshFromConfig(ctx, plan.Refresh)
			diags.Append(d...)
			rotate.Refresh = client.Set(refresh)
		}
	}
	return rotate, diags
}

func refreshFromConfig(ctx context.Context, obj types.Object) (client.OAuthRefreshWrite, diag.Diagnostics) {
	var diags diag.Diagnostics
	var rm oauthRefreshModel
	diags.Append(obj.As(ctx, &rm, basetypes.ObjectAsOptions{})...)
	out := client.OAuthRefreshWrite{
		TokenEndpoint: rm.TokenEndpoint.ValueString(),
		ClientID:      rm.ClientID.ValueString(),
		RefreshToken:  rm.RefreshToken.ValueString(),
		Scope:         rm.Scope.ValueString(),
		Resource:      rm.Resource.ValueString(),
	}
	if !rm.TokenEndpointAuth.IsNull() && !rm.TokenEndpointAuth.IsUnknown() {
		var tea tokenEndpointAuthModel
		diags.Append(rm.TokenEndpointAuth.As(ctx, &tea, basetypes.ObjectAsOptions{})...)
		out.TokenEndpointAuth = &client.TokenEndpointAuthWrite{
			Type:         tea.Type.ValueString(),
			ClientSecret: tea.ClientSecret.ValueString(),
		}
	}
	return out, diags
}

func credentialToState(plan vaultCredentialModel, cred *client.Credential) vaultCredentialModel {
	out := plan
	out.ID = types.StringValue(cred.ID)
	out.Object = types.StringValue(cred.Object)
	out.CreatedAt = types.Int64Value(cred.CreatedAt)
	out.UpdatedAt = types.Int64Value(cred.UpdatedAt)
	out.VaultID = types.StringValue(cred.VaultID)
	out.AuthType = types.StringValue(cred.AuthType)
	out.MCPServerURL = types.StringValue(cred.MCPServerURL)
	if cred.Name != nil && *cred.Name != "" {
		out.Name = types.StringValue(*cred.Name)
	} else {
		out.Name = types.StringNull()
	}
	out.ExpiresAt = stringPtrValue(cred.ExpiresAt)
	out.Token = types.StringNull()
	out.AccessToken = types.StringNull()
	if !plan.Refresh.IsNull() && !plan.Refresh.IsUnknown() {
		out.Refresh = stripRefreshSecrets(plan.Refresh)
	} else {
		out.Refresh = types.ObjectNull(refreshAttrTypes)
	}
	return out
}

func stripRefreshSecrets(obj types.Object) types.Object {
	attrs := obj.Attributes()
	if attrs == nil {
		return types.ObjectNull(refreshAttrTypes)
	}
	out := map[string]attr.Value{}
	for k, v := range attrs {
		out[k] = v
	}
	out["refresh_token"] = types.StringNull()
	if _, ok := out["scope"]; !ok {
		out["scope"] = types.StringNull()
	}
	if _, ok := out["resource"]; !ok {
		out["resource"] = types.StringNull()
	}
	if tea, ok := out["token_endpoint_auth"].(types.Object); ok && !tea.IsNull() {
		teaAttrs := tea.Attributes()
		teaAttrs["client_secret"] = types.StringNull()
		n, _ := types.ObjectValue(tokenEndpointAuthAttrTypes, teaAttrs)
		out["token_endpoint_auth"] = n
	}
	n, _ := types.ObjectValue(refreshAttrTypes, out)
	return n
}

func stringPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}
