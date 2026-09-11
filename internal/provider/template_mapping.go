// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/MemoryStore/terraform-provider-openaiagents/internal/client"
)

var networkAttrTypes = map[string]attr.Type{
	"access":          types.StringType,
	"allowed_domains": types.ListType{ElemType: types.StringType},
}

var packagesAttrTypes = map[string]attr.Type{
	"python": types.ListType{ElemType: types.StringType},
	"system": types.ListType{ElemType: types.StringType},
	"npm":    types.ListType{ElemType: types.StringType},
}

var fileAttrTypes = map[string]attr.Type{
	"type":    types.StringType,
	"path":    types.StringType,
	"file_id": types.StringType,
	"data":    types.StringType,
}

var skillAttrTypes = map[string]attr.Type{
	"type":              types.StringType,
	"name":              types.StringType,
	"description":       types.StringType,
	"skill_id":          types.StringType,
	"version":           types.StringType,
	"source_data":       types.StringType,
	"source_media_type": types.StringType,
}

var pluginAttrTypes = map[string]attr.Type{
	"type":              types.StringType,
	"name":              types.StringType,
	"description":       types.StringType,
	"source_data":       types.StringType,
	"source_media_type": types.StringType,
}

var setupAttrTypes = map[string]attr.Type{
	"command": types.StringType,
	"cwd":     types.StringType,
}

func templateWriteFrom(ctx context.Context, plan, state, config templateModel, create bool) (client.TemplateWrite, diag.Diagnostics) {
	var diags diag.Diagnostics
	write := client.TemplateWrite{SendConfidential: create}
	write.Name = stringOptional(plan.Name, state.Name)
	if !plan.CapabilityDirectories.IsNull() && !plan.CapabilityDirectories.IsUnknown() {
		var dirs []string
		diags.Append(plan.CapabilityDirectories.ElementsAs(ctx, &dirs, false)...)
		write.CapabilityDirectories = client.Set(dirs)
	} else if plan.CapabilityDirectories.IsNull() && !state.CapabilityDirectories.IsNull() && !state.CapabilityDirectories.IsUnknown() {
		write.CapabilityDirectories = client.Null[[]string]()
	}
	if !plan.Network.IsNull() && !plan.Network.IsUnknown() {
		n, d := networkFromObject(ctx, plan.Network)
		diags.Append(d...)
		write.Network = client.Set(n)
	} else if plan.Network.IsNull() && !state.Network.IsNull() && !state.Network.IsUnknown() {
		write.Network = client.Null[client.Network]()
	}
	if !plan.Packages.IsNull() && !plan.Packages.IsUnknown() {
		p, d := packagesFromObject(ctx, plan.Packages)
		diags.Append(d...)
		write.Packages = client.Set(p)
	} else if plan.Packages.IsNull() && !state.Packages.IsNull() && !state.Packages.IsUnknown() {
		write.Packages = client.Null[client.Packages]()
	}

	sendEnv := create || revisionChanged(plan.EnvRevision, state.EnvRevision)
	sendSetup := create || revisionChanged(plan.SetupCommandsRevision, state.SetupCommandsRevision)
	sendFiles := create || revisionChanged(plan.FilesRevision, state.FilesRevision)
	sendSkills := create || revisionChanged(plan.SkillsRevision, state.SkillsRevision)
	sendPlugins := create || revisionChanged(plan.PluginsRevision, state.PluginsRevision)
	if sendEnv || sendSetup || sendFiles || sendSkills || sendPlugins {
		write.SendConfidential = true
	}

	if sendEnv {
		if config.Env.IsNull() || config.Env.IsUnknown() {
			if !create && revisionChanged(plan.EnvRevision, state.EnvRevision) {
				diags.AddError("Missing env values", "env_revision changed but env was not provided in configuration")
			}
		} else {
			m := map[string]string{}
			diags.Append(config.Env.ElementsAs(ctx, &m, false)...)
			write.Env = client.Set(m)
		}
	}
	if sendSetup {
		if config.SetupCommands.IsNull() || config.SetupCommands.IsUnknown() {
			if !create {
				diags.AddError("Missing setup_commands", "setup_commands_revision changed but setup_commands was not provided in configuration")
			}
		} else {
			cmds, d := setupCommandsFromConfig(ctx, config.SetupCommands)
			diags.Append(d...)
			write.SetupCommands = client.Set(cmds)
		}
	}
	if sendFiles {
		if config.Files.IsNull() || config.Files.IsUnknown() {
			if !create {
				diags.AddError("Missing files", "files_revision changed but files was not provided in configuration")
			}
		} else {
			files, d := filesFromConfig(ctx, config.Files, true)
			diags.Append(d...)
			write.Files = client.Set(files)
		}
	}
	if sendSkills {
		if config.Skills.IsNull() || config.Skills.IsUnknown() {
			if !create {
				diags.AddError("Missing skills", "skills_revision changed but skills was not provided in configuration")
			}
		} else {
			skills, d := skillsFromConfig(ctx, config.Skills, true)
			diags.Append(d...)
			write.Skills = client.Set(skills)
		}
	}
	if sendPlugins {
		if config.Plugins.IsNull() || config.Plugins.IsUnknown() {
			if !create {
				diags.AddError("Missing plugins", "plugins_revision changed but plugins was not provided in configuration")
			}
		} else {
			plugins, d := pluginsFromConfig(ctx, config.Plugins, true)
			diags.Append(d...)
			write.Plugins = client.Set(plugins)
		}
	}
	return write, diags
}

func networkFromObject(ctx context.Context, obj types.Object) (client.Network, diag.Diagnostics) {
	var diags diag.Diagnostics
	var n struct {
		Access         types.String `tfsdk:"access"`
		AllowedDomains types.List   `tfsdk:"allowed_domains"`
	}
	diags.Append(obj.As(ctx, &n, basetypes.ObjectAsOptions{})...)
	out := client.Network{Access: n.Access.ValueString()}
	if !n.AllowedDomains.IsNull() && !n.AllowedDomains.IsUnknown() {
		var domains []string
		diags.Append(n.AllowedDomains.ElementsAs(ctx, &domains, false)...)
		out.AllowedDomains = domains
	}
	return out, diags
}

func packagesFromObject(ctx context.Context, obj types.Object) (client.Packages, diag.Diagnostics) {
	var diags diag.Diagnostics
	var p struct {
		Python types.List `tfsdk:"python"`
		System types.List `tfsdk:"system"`
		NPM    types.List `tfsdk:"npm"`
	}
	diags.Append(obj.As(ctx, &p, basetypes.ObjectAsOptions{})...)
	out := client.Packages{}
	if !p.Python.IsNull() {
		diags.Append(p.Python.ElementsAs(ctx, &out.Python, false)...)
	}
	if !p.System.IsNull() {
		diags.Append(p.System.ElementsAs(ctx, &out.System, false)...)
	}
	if !p.NPM.IsNull() {
		diags.Append(p.NPM.ElementsAs(ctx, &out.NPM, false)...)
	}
	return out, diags
}

func setupCommandsFromConfig(ctx context.Context, list types.List) ([]client.SetupCommand, diag.Diagnostics) {
	var diags diag.Diagnostics
	var elems []types.Object
	diags.Append(list.ElementsAs(ctx, &elems, false)...)
	out := make([]client.SetupCommand, 0, len(elems))
	for _, elem := range elems {
		var c struct {
			Command types.String `tfsdk:"command"`
			Cwd     types.String `tfsdk:"cwd"`
		}
		diags.Append(elem.As(ctx, &c, basetypes.ObjectAsOptions{})...)
		out = append(out, client.SetupCommand{Command: c.Command.ValueString(), Cwd: c.Cwd.ValueString()})
	}
	return out, diags
}

func filesFromConfig(ctx context.Context, list types.List, includeData bool) ([]client.TemplateFile, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}
	var elems []types.Object
	diags.Append(list.ElementsAs(ctx, &elems, false)...)
	out := make([]client.TemplateFile, 0, len(elems))
	for _, elem := range elems {
		var f struct {
			Type   types.String `tfsdk:"type"`
			Path   types.String `tfsdk:"path"`
			FileID types.String `tfsdk:"file_id"`
			Data   types.String `tfsdk:"data"`
		}
		diags.Append(elem.As(ctx, &f, basetypes.ObjectAsOptions{})...)
		item := client.TemplateFile{Type: f.Type.ValueString(), Path: f.Path.ValueString(), FileID: f.FileID.ValueString()}
		if includeData {
			item.Data = f.Data.ValueString()
		}
		out = append(out, item)
	}
	return out, diags
}

func skillsFromConfig(ctx context.Context, list types.List, includeData bool) ([]client.TemplateSkill, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}
	var elems []types.Object
	diags.Append(list.ElementsAs(ctx, &elems, false)...)
	out := make([]client.TemplateSkill, 0, len(elems))
	for _, elem := range elems {
		var s struct {
			Type            types.String `tfsdk:"type"`
			Name            types.String `tfsdk:"name"`
			Description     types.String `tfsdk:"description"`
			SkillID         types.String `tfsdk:"skill_id"`
			Version         types.String `tfsdk:"version"`
			SourceData      types.String `tfsdk:"source_data"`
			SourceMediaType types.String `tfsdk:"source_media_type"`
		}
		diags.Append(elem.As(ctx, &s, basetypes.ObjectAsOptions{})...)
		item := client.TemplateSkill{
			Type:        s.Type.ValueString(),
			Name:        s.Name.ValueString(),
			Description: s.Description.ValueString(),
			SkillID:     s.SkillID.ValueString(),
			Version:     s.Version.ValueString(),
		}
		if includeData && s.SourceData.ValueString() != "" {
			media := s.SourceMediaType.ValueString()
			if media == "" {
				media = "application/zip"
			}
			item.Source = &client.ArchiveSource{Type: "base64", MediaType: media, Data: s.SourceData.ValueString()}
		}
		out = append(out, item)
	}
	return out, diags
}

func pluginsFromConfig(ctx context.Context, list types.List, includeData bool) ([]client.TemplatePlugin, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}
	var elems []types.Object
	diags.Append(list.ElementsAs(ctx, &elems, false)...)
	out := make([]client.TemplatePlugin, 0, len(elems))
	for _, elem := range elems {
		var p struct {
			Type            types.String `tfsdk:"type"`
			Name            types.String `tfsdk:"name"`
			Description     types.String `tfsdk:"description"`
			SourceData      types.String `tfsdk:"source_data"`
			SourceMediaType types.String `tfsdk:"source_media_type"`
		}
		diags.Append(elem.As(ctx, &p, basetypes.ObjectAsOptions{})...)
		item := client.TemplatePlugin{Type: p.Type.ValueString(), Name: p.Name.ValueString(), Description: p.Description.ValueString()}
		if includeData && p.SourceData.ValueString() != "" {
			media := p.SourceMediaType.ValueString()
			if media == "" {
				media = "application/zip"
			}
			item.Source = &client.ArchiveSource{Type: "base64", MediaType: media, Data: p.SourceData.ValueString()}
		}
		out = append(out, item)
	}
	return out, diags
}

func templateToState(ctx context.Context, prior templateModel, tpl *client.EnvironmentTemplate) (templateModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := prior
	out.ID = types.StringValue(tpl.ID)
	out.Object = types.StringValue(tpl.Object)
	out.CreatedAt = types.Int64Value(tpl.CreatedAt)
	out.UpdatedAt = types.Int64Value(tpl.UpdatedAt)
	out.Name = stringPtrValue(tpl.Name)
	out.Env = types.MapNull(types.StringType)

	if prior.CapabilityDirectories.IsNull() || prior.CapabilityDirectories.IsUnknown() {
		out.CapabilityDirectories = types.ListNull(types.StringType)
	} else if tpl.CapabilityDirectories != nil {
		lv, d := types.ListValueFrom(ctx, types.StringType, tpl.CapabilityDirectories)
		diags.Append(d...)
		out.CapabilityDirectories = lv
	}

	if prior.Network.IsNull() || prior.Network.IsUnknown() {
		out.Network = types.ObjectNull(networkAttrTypes)
	} else if tpl.Network != nil {
		domains := types.ListNull(types.StringType)
		if tpl.Network.AllowedDomains != nil {
			lv, d := types.ListValueFrom(ctx, types.StringType, tpl.Network.AllowedDomains)
			diags.Append(d...)
			domains = lv
		}
		obj, d := types.ObjectValue(networkAttrTypes, map[string]attr.Value{
			"access":          nullIfEmpty(tpl.Network.Access),
			"allowed_domains": domains,
		})
		diags.Append(d...)
		out.Network = obj
	}

	if prior.Packages.IsNull() || prior.Packages.IsUnknown() {
		out.Packages = types.ObjectNull(packagesAttrTypes)
	} else if tpl.Packages != nil {
		obj, d := packagesToObject(ctx, *tpl.Packages)
		diags.Append(d...)
		out.Packages = obj
	}

	if tpl.EnvKeys != nil {
		lv, d := types.ListValueFrom(ctx, types.StringType, tpl.EnvKeys)
		diags.Append(d...)
		out.EnvKeys = lv
	} else {
		out.EnvKeys = types.ListNull(types.StringType)
	}

	if prior.Files.IsNull() || prior.Files.IsUnknown() {
		out.Files = types.ListNull(types.ObjectType{AttrTypes: fileAttrTypes})
	} else {
		files := make([]attr.Value, 0, len(tpl.Files))
		for _, f := range tpl.Files {
			obj, d := types.ObjectValue(fileAttrTypes, map[string]attr.Value{
				"type":    types.StringValue(f.Type),
				"path":    types.StringValue(f.Path),
				"file_id": nullIfEmpty(f.FileID),
				"data":    types.StringNull(),
			})
			diags.Append(d...)
			files = append(files, obj)
		}
		lv, d := types.ListValue(types.ObjectType{AttrTypes: fileAttrTypes}, files)
		diags.Append(d...)
		out.Files = lv
	}

	if prior.Skills.IsNull() || prior.Skills.IsUnknown() {
		out.Skills = types.ListNull(types.ObjectType{AttrTypes: skillAttrTypes})
	} else {
		skills := make([]attr.Value, 0, len(tpl.Skills))
		for _, s := range tpl.Skills {
			media := types.StringNull()
			if s.Source != nil {
				media = nullIfEmpty(s.Source.MediaType)
			}
			obj, d := types.ObjectValue(skillAttrTypes, map[string]attr.Value{
				"type":              types.StringValue(s.Type),
				"name":              nullIfEmpty(s.Name),
				"description":       nullIfEmpty(s.Description),
				"skill_id":          nullIfEmpty(s.SkillID),
				"version":           nullIfEmpty(s.Version),
				"source_data":       types.StringNull(),
				"source_media_type": media,
			})
			diags.Append(d...)
			skills = append(skills, obj)
		}
		lv, d := types.ListValue(types.ObjectType{AttrTypes: skillAttrTypes}, skills)
		diags.Append(d...)
		out.Skills = lv
	}

	if prior.Plugins.IsNull() || prior.Plugins.IsUnknown() {
		out.Plugins = types.ListNull(types.ObjectType{AttrTypes: pluginAttrTypes})
	} else {
		plugins := make([]attr.Value, 0, len(tpl.Plugins))
		for _, p := range tpl.Plugins {
			media := types.StringNull()
			if p.Source != nil {
				media = nullIfEmpty(p.Source.MediaType)
			}
			obj, d := types.ObjectValue(pluginAttrTypes, map[string]attr.Value{
				"type":              types.StringValue(p.Type),
				"name":              nullIfEmpty(p.Name),
				"description":       nullIfEmpty(p.Description),
				"source_data":       types.StringNull(),
				"source_media_type": media,
			})
			diags.Append(d...)
			plugins = append(plugins, obj)
		}
		lv, d := types.ListValue(types.ObjectType{AttrTypes: pluginAttrTypes}, plugins)
		diags.Append(d...)
		out.Plugins = lv
	}

	// The hosted API does not return setup_commands. Keep configured cwd and
	// null the write-only command body so create does not produce an
	// inconsistent sensitive value.
	out.SetupCommands = setupCommandsToState(prior.SetupCommands)

	return out, diags
}

func packagesToObject(ctx context.Context, p client.Packages) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	python, d := optionalStringList(ctx, p.Python)
	diags.Append(d...)
	system, d := optionalStringList(ctx, p.System)
	diags.Append(d...)
	npm, d := optionalStringList(ctx, p.NPM)
	diags.Append(d...)
	obj, d := types.ObjectValue(packagesAttrTypes, map[string]attr.Value{
		"python": python,
		"system": system,
		"npm":    npm,
	})
	diags.Append(d...)
	return obj, diags
}

func setupCommandsToState(prior types.List) types.List {
	if prior.IsNull() || prior.IsUnknown() {
		return types.ListNull(types.ObjectType{AttrTypes: setupAttrTypes})
	}
	elems := prior.Elements()
	out := make([]attr.Value, 0, len(elems))
	for _, elem := range elems {
		obj, ok := elem.(types.Object)
		if !ok || obj.IsNull() {
			continue
		}
		cwd := types.StringNull()
		if v, exists := obj.Attributes()["cwd"]; exists {
			if s, ok := v.(types.String); ok && !s.IsUnknown() {
				cwd = s
			}
		}
		n, _ := types.ObjectValue(setupAttrTypes, map[string]attr.Value{
			"command": types.StringNull(),
			"cwd":     cwd,
		})
		out = append(out, n)
	}
	lv, _ := types.ListValue(types.ObjectType{AttrTypes: setupAttrTypes}, out)
	return lv
}

func optionalStringList(ctx context.Context, values []string) (types.List, diag.Diagnostics) {
	if values == nil {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(ctx, types.StringType, values)
}
