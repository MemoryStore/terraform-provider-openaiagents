// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/MemoryStore/terraform-provider-openaiagents/internal/client"
)

var _ resource.Resource = &EnvironmentTemplateResource{}
var _ resource.ResourceWithImportState = &EnvironmentTemplateResource{}
var _ resource.ResourceWithModifyPlan = &EnvironmentTemplateResource{}

// EnvironmentTemplateResource manages a hosted environment template.
type EnvironmentTemplateResource struct {
	client *client.Client
}

func NewEnvironmentTemplateResource() resource.Resource { return &EnvironmentTemplateResource{} }

type templateModel struct {
	ID                    types.String `tfsdk:"id"`
	Object                types.String `tfsdk:"object"`
	CreatedAt             types.Int64  `tfsdk:"created_at"`
	UpdatedAt             types.Int64  `tfsdk:"updated_at"`
	Name                  types.String `tfsdk:"name"`
	CapabilityDirectories types.List   `tfsdk:"capability_directories"`
	Network               types.Object `tfsdk:"network"`
	Packages              types.Object `tfsdk:"packages"`
	Files                 types.List   `tfsdk:"files"`
	Skills                types.List   `tfsdk:"skills"`
	Plugins               types.List   `tfsdk:"plugins"`
	Env                   types.Map    `tfsdk:"env"`
	SetupCommands         types.List   `tfsdk:"setup_commands"`
	EnvRevision           types.Int64  `tfsdk:"env_revision"`
	SetupCommandsRevision types.Int64  `tfsdk:"setup_commands_revision"`
	FilesRevision         types.Int64  `tfsdk:"files_revision"`
	SkillsRevision        types.Int64  `tfsdk:"skills_revision"`
	PluginsRevision       types.Int64  `tfsdk:"plugins_revision"`
	EnvKeys               types.List   `tfsdk:"env_keys"`
}

func (r *EnvironmentTemplateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_template"
}

func (r *EnvironmentTemplateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A reusable OpenAI-hosted environment template (`POST /v1/agents/environments/templates`). " +
			"Creating a template does not create a session or a running environment. " +
			"Environment values, setup-command bodies, and inline/archive bytes are write-only; the API does not read them back. " +
			"Use the corresponding `*_revision` attributes to deploy changes. Drift detection for those confidential inputs is revision-based only.",
		Attributes: map[string]schema.Attribute{
			"id":                     schema.StringAttribute{Computed: true, MarkdownDescription: "Remote template ID.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"object":                 schema.StringAttribute{Computed: true, MarkdownDescription: "Object type."},
			"created_at":             schema.Int64Attribute{Computed: true, MarkdownDescription: "Unix create timestamp.", PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"updated_at":             schema.Int64Attribute{Computed: true, MarkdownDescription: "Unix update timestamp."},
			"name":                   schema.StringAttribute{Optional: true, MarkdownDescription: "Display name."},
			"capability_directories": schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Absolute capability directories discovered by the harness."},
			"network": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Network policy for hosted environments.",
				Attributes: map[string]schema.Attribute{
					"access":          schema.StringAttribute{Optional: true, MarkdownDescription: "`enabled`, `disabled`, or `restricted`."},
					"allowed_domains": schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Exact host names allowed when access is `restricted`."},
				},
			},
			"packages": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Packages to install.",
				Attributes: map[string]schema.Attribute{
					"python": schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Python packages."},
					"system": schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "System packages."},
					"npm":    schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Global npm packages."},
				},
			},
			"files": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Workspace files. Inline `data` is write-only. Changing path, type, or `file_id` also sends the group when `data` remains in configuration; otherwise increment `files_revision`.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"type":    schema.StringAttribute{Required: true, MarkdownDescription: "`file_id` or `inline`."},
					"path":    schema.StringAttribute{Required: true, MarkdownDescription: "Destination path under `/workspace`."},
					"file_id": schema.StringAttribute{Optional: true, MarkdownDescription: "Files API ID when `type` is `file_id`."},
					"data":    schema.StringAttribute{Optional: true, WriteOnly: true, Sensitive: true, MarkdownDescription: "Base64 inline content. Write-only."},
				}},
			},
			"skills": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Skills: inline archives or Skills API references. Archive bytes are write-only.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"type":              schema.StringAttribute{Required: true, MarkdownDescription: "`inline` or `skill_reference`."},
					"name":              schema.StringAttribute{Optional: true, MarkdownDescription: "Inline skill or plugin name."},
					"description":       schema.StringAttribute{Optional: true, MarkdownDescription: "Inline description."},
					"skill_id":          schema.StringAttribute{Optional: true, MarkdownDescription: "Existing Skills API object ID."},
					"version":           schema.StringAttribute{Optional: true, MarkdownDescription: "Pinned skill version. Omitting it or using `latest` allows future sessions to resolve different content."},
					"source_data":       schema.StringAttribute{Optional: true, WriteOnly: true, Sensitive: true, MarkdownDescription: "Base64 ZIP archive. Write-only."},
					"source_media_type": schema.StringAttribute{Optional: true, MarkdownDescription: "Archive media type. Defaults to `application/zip`."},
				}},
			},
			"plugins": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Inline plugin ZIP archives. Archive bytes are write-only.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"type":              schema.StringAttribute{Required: true, MarkdownDescription: "Always `inline` for this release."},
					"name":              schema.StringAttribute{Optional: true, MarkdownDescription: "Plugin name; must match the manifest."},
					"description":       schema.StringAttribute{Optional: true, MarkdownDescription: "Plugin description; must match the manifest."},
					"source_data":       schema.StringAttribute{Optional: true, WriteOnly: true, Sensitive: true, MarkdownDescription: "Base64 ZIP archive. Write-only."},
					"source_media_type": schema.StringAttribute{Optional: true, MarkdownDescription: "Archive media type. Defaults to `application/zip`."},
				}},
			},
			"env": schema.MapAttribute{
				Optional:            true,
				WriteOnly:           true,
				Sensitive:           true,
				ElementType:         types.StringType,
				MarkdownDescription: "Environment variable values. Write-only; not read back. Change `env_revision` to send value updates. Adding or removing keys while `env` remains in configuration also sends `env`.",
			},
			"setup_commands": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Setup commands. Command bodies are write-only. Changing `cwd` also sends the group when `command` remains in configuration; otherwise increment `setup_commands_revision`.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"command": schema.StringAttribute{Optional: true, WriteOnly: true, Sensitive: true, MarkdownDescription: "Shell command body. Write-only."},
					"cwd":     schema.StringAttribute{Optional: true, MarkdownDescription: "Working directory. The API does not read setup commands back; drift detection is revision-based."},
				}},
			},
			"env_revision":            schema.Int64Attribute{Optional: true, MarkdownDescription: "Non-secret revision. Increment to send `env` values."},
			"setup_commands_revision": schema.Int64Attribute{Optional: true, MarkdownDescription: "Non-secret revision. Increment to send setup command bodies."},
			"files_revision":          schema.Int64Attribute{Optional: true, MarkdownDescription: "Non-secret revision. Increment to upload inline file bytes."},
			"skills_revision":         schema.Int64Attribute{Optional: true, MarkdownDescription: "Non-secret revision. Increment to upload skill archives."},
			"plugins_revision":        schema.Int64Attribute{Optional: true, MarkdownDescription: "Non-secret revision. Increment to upload plugin archives."},
			"env_keys":                schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Environment variable names returned by the API, if any. Values are never returned."},
		},
	}
}

func (r *EnvironmentTemplateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *EnvironmentTemplateResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || req.State.Raw.IsNull() {
		return
	}
	var plan, state, config templateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !revisionChanged(plan.EnvRevision, state.EnvRevision) && envKeysChanged(ctx, config.Env, state.EnvKeys) {
		resp.Diagnostics.AddError(
			"env_revision required",
			"env keys changed without env_revision; increment env_revision and keep env in configuration",
		)
	}
}

func (r *EnvironmentTemplateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan templateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var config templateModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	write, d := templateWriteFrom(ctx, plan, templateModel{}, config, true)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	tpl, err := r.client.CreateTemplate(ctx, write)
	if err != nil {
		addClientError(&resp.Diagnostics, "Unable to create environment template", err)
		return
	}
	state, d := templateToState(ctx, plan, tpl)
	resp.Diagnostics.Append(d...)
	if keys, ok := envMapKeys(ctx, config.Env); ok && (state.EnvKeys.IsNull() || len(tpl.EnvKeys) == 0) {
		lv, kd := types.ListValueFrom(ctx, types.StringType, keys)
		resp.Diagnostics.Append(kd...)
		state.EnvKeys = lv
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *EnvironmentTemplateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state templateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tpl, err := r.client.GetTemplate(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		addClientError(&resp.Diagnostics, "Unable to read environment template", err)
		return
	}
	next, d := templateToState(ctx, state, tpl)
	resp.Diagnostics.Append(d...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *EnvironmentTemplateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state templateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	var config templateModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	write, d := templateWriteFrom(ctx, plan, state, config, false)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	tpl, err := r.client.UpdateTemplate(ctx, state.ID.ValueString(), write)
	if err != nil {
		addClientError(&resp.Diagnostics, "Unable to update environment template", err)
		return
	}
	next, d := templateToState(ctx, plan, tpl)
	resp.Diagnostics.Append(d...)
	if keys, ok := envMapKeys(ctx, config.Env); ok && write.Env.Present && (next.EnvKeys.IsNull() || len(tpl.EnvKeys) == 0) {
		lv, kd := types.ListValueFrom(ctx, types.StringType, keys)
		resp.Diagnostics.Append(kd...)
		next.EnvKeys = lv
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *EnvironmentTemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state templateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteTemplate(ctx, state.ID.ValueString()); err != nil {
		addClientError(&resp.Diagnostics, "Unable to delete environment template", err)
	}
}

func (r *EnvironmentTemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if err := validateImportID(req.ID); err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func revisionChanged(plan, state types.Int64) bool {
	if plan.IsNull() && state.IsNull() {
		return false
	}
	if plan.IsNull() || state.IsNull() {
		return true
	}
	return plan.ValueInt64() != state.ValueInt64()
}
