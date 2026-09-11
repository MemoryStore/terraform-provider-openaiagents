// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"sort"
	"strings"

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

	sendEnv := create || stringRevisionChanged(plan.EnvRevision, state.EnvRevision)
	sendSetup := create || stringRevisionChanged(plan.SetupCommandsRevision, state.SetupCommandsRevision)
	sendFiles := create || stringRevisionChanged(plan.FilesRevision, state.FilesRevision)
	sendSkills := create || stringRevisionChanged(plan.SkillsRevision, state.SkillsRevision)
	sendPlugins := create || stringRevisionChanged(plan.PluginsRevision, state.PluginsRevision)
	if !create && !sendSetup && gatedListPublicChanged(plan.SetupCommands, state.SetupCommands, []string{"cwd"}) {
		sendSetup = true
	}
	if !create && !sendFiles && gatedListPublicChanged(plan.Files, state.Files, []string{"type", "path", "file_id"}) {
		sendFiles = true
	}
	if !create && !sendSkills && gatedListPublicChanged(plan.Skills, state.Skills, []string{"type", "name", "description", "skill_id", "version", "source_media_type"}) {
		sendSkills = true
	}
	if !create && !sendPlugins && gatedListPublicChanged(plan.Plugins, state.Plugins, []string{"type", "name", "description", "source_media_type"}) {
		sendPlugins = true
	}
	if !create && !sendEnv && envKeysChanged(ctx, config.Env, state.EnvKeys) {
		sendEnv = true
	}
	if sendEnv || sendSetup || sendFiles || sendSkills || sendPlugins {
		write.SendConfidential = true
	}

	if sendEnv {
		if config.Env.IsNull() || config.Env.IsUnknown() {
			if !create {
				diags.AddError("Missing env values", "env keys or env_revision changed; increment env_revision and provide env in configuration")
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
				diags.AddError("Missing setup_commands", "setup_commands changed; increment setup_commands_revision and provide setup_commands in configuration")
			}
		} else {
			cmds, d := setupCommandsFromConfig(ctx, config.SetupCommands)
			diags.Append(d...)
			if !create && setupCommandsMissingBody(cmds) {
				diags.AddError("Missing setup_commands", "setup_commands cwd or entries changed; increment setup_commands_revision and include command bodies in configuration")
			} else {
				write.SetupCommands = client.Set(cmds)
			}
		}
	}
	if sendFiles {
		if config.Files.IsNull() || config.Files.IsUnknown() {
			if !create {
				diags.AddError("Missing files", "files changed; increment files_revision and provide files in configuration")
			}
		} else {
			files, d := filesFromConfig(ctx, config.Files, true)
			diags.Append(d...)
			if !create && inlineFilesMissingData(files) {
				diags.AddError("Missing files data", "files path or metadata changed; increment files_revision and include files[].data in configuration")
			} else {
				write.Files = client.Set(files)
			}
		}
	}
	if sendSkills {
		if config.Skills.IsNull() || config.Skills.IsUnknown() {
			if !create {
				diags.AddError("Missing skills", "skills changed; increment skills_revision and provide skills in configuration")
			}
		} else {
			skills, d := skillsFromConfig(ctx, config.Skills, true)
			diags.Append(d...)
			if !create && inlineSkillsMissingData(skills) {
				diags.AddError("Missing skills data", "skills metadata changed; increment skills_revision and include source_data for inline skills")
			} else {
				write.Skills = client.Set(skills)
			}
		}
	}
	if sendPlugins {
		if config.Plugins.IsNull() || config.Plugins.IsUnknown() {
			if !create {
				diags.AddError("Missing plugins", "plugins changed; increment plugins_revision and provide plugins in configuration")
			}
		} else {
			plugins, d := pluginsFromConfig(ctx, config.Plugins, true)
			diags.Append(d...)
			if !create && inlinePluginsMissingData(plugins) {
				diags.AddError("Missing plugins data", "plugins metadata changed; increment plugins_revision and include source_data in configuration")
			} else {
				write.Plugins = client.Set(plugins)
			}
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

func attrKnown(v interface {
	IsNull() bool
	IsUnknown() bool
}) bool {
	return !v.IsNull() && !v.IsUnknown()
}

func templateImport(prior templateModel) bool {
	return prior.Name.IsNull() &&
		prior.CapabilityDirectories.IsNull() &&
		prior.Network.IsNull() &&
		prior.Packages.IsNull() &&
		prior.Files.IsNull() &&
		prior.Skills.IsNull() &&
		prior.Plugins.IsNull() &&
		prior.SetupCommands.IsNull()
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
	importing := templateImport(prior)

	keepDirs := importing || attrKnown(prior.CapabilityDirectories)
	if keepDirs && len(tpl.CapabilityDirectories) > 0 {
		lv, d := types.ListValueFrom(ctx, types.StringType, tpl.CapabilityDirectories)
		diags.Append(d...)
		out.CapabilityDirectories = lv
	} else if !keepDirs {
		out.CapabilityDirectories = types.ListNull(types.StringType)
	}

	keepNetwork := importing || attrKnown(prior.Network)
	if !keepNetwork {
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

	keepPackages := importing || attrKnown(prior.Packages)
	if !keepPackages {
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
	} else if !prior.EnvKeys.IsNull() && !prior.EnvKeys.IsUnknown() {
		out.EnvKeys = prior.EnvKeys
	} else {
		out.EnvKeys = types.ListNull(types.StringType)
	}

	keepFiles := importing || attrKnown(prior.Files)
	if !keepFiles || (importing && len(tpl.Files) == 0) {
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

	keepSkills := importing || attrKnown(prior.Skills)
	if !keepSkills || (importing && len(tpl.Skills) == 0) {
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

	keepPlugins := importing || attrKnown(prior.Plugins)
	if !keepPlugins || (importing && len(tpl.Plugins) == 0) {
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

func gatedListPublicChanged(plan, state types.List, keys []string) bool {
	if plan.IsUnknown() || state.IsUnknown() {
		return false
	}
	if plan.IsNull() && state.IsNull() {
		return false
	}
	if plan.IsNull() {
		return false
	}
	if state.IsNull() {
		return true
	}
	pe := plan.Elements()
	se := state.Elements()
	if len(pe) != len(se) {
		return true
	}
	for i := range pe {
		po, pok := pe[i].(types.Object)
		so, sok := se[i].(types.Object)
		if !pok || !sok || po.IsUnknown() || so.IsUnknown() {
			continue
		}
		for _, key := range keys {
			if publicStringAttr(po.Attributes()[key]) != publicStringAttr(so.Attributes()[key]) {
				return true
			}
		}
	}
	return false
}

func envMapKeys(ctx context.Context, env types.Map) ([]string, bool) {
	if env.IsNull() || env.IsUnknown() {
		return nil, false
	}
	m := map[string]string{}
	if diags := env.ElementsAs(ctx, &m, false); diags.HasError() {
		return nil, false
	}
	return mapKeys(m), true
}

func envKeysChanged(ctx context.Context, configEnv types.Map, stateKeys types.List) bool {
	if configEnv.IsNull() || configEnv.IsUnknown() || stateKeys.IsNull() || stateKeys.IsUnknown() {
		return false
	}
	cfg := map[string]string{}
	if diags := configEnv.ElementsAs(ctx, &cfg, false); diags.HasError() {
		return false
	}
	var keys []string
	if diags := stateKeys.ElementsAs(ctx, &keys, false); diags.HasError() {
		return false
	}
	return !stringSetEqual(mapKeys(cfg), keys)
}

func mapKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func stringSetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as := append([]string(nil), a...)
	bs := append([]string(nil), b...)
	sort.Strings(as)
	sort.Strings(bs)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

func setupCommandsMissingBody(cmds []client.SetupCommand) bool {
	if len(cmds) == 0 {
		return true
	}
	for _, c := range cmds {
		if strings.TrimSpace(c.Command) == "" {
			return true
		}
	}
	return false
}

func inlineFilesMissingData(files []client.TemplateFile) bool {
	for _, f := range files {
		if f.Type == "inline" && strings.TrimSpace(f.Data) == "" {
			return true
		}
	}
	return false
}

func inlineSkillsMissingData(skills []client.TemplateSkill) bool {
	for _, s := range skills {
		if s.Type == "inline" && (s.Source == nil || strings.TrimSpace(s.Source.Data) == "") {
			return true
		}
	}
	return false
}

func inlinePluginsMissingData(plugins []client.TemplatePlugin) bool {
	for _, p := range plugins {
		if p.Source == nil || strings.TrimSpace(p.Source.Data) == "" {
			return true
		}
	}
	return false
}

func optionalStringList(ctx context.Context, values []string) (types.List, diag.Diagnostics) {
	if values == nil {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(ctx, types.StringType, values)
}
