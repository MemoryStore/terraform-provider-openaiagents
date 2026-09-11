// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/MemoryStore/terraform-provider-openaiagents/internal/client"
)

type agentModel struct {
	ID           types.String `tfsdk:"id"`
	Object       types.String `tfsdk:"object"`
	CreatedAt    types.Int64  `tfsdk:"created_at"`
	UpdatedAt    types.Int64  `tfsdk:"updated_at"`
	Model        types.String `tfsdk:"model"`
	Instructions types.String `tfsdk:"instructions"`
	Name         types.String `tfsdk:"name"`
	Metadata     types.Map    `tfsdk:"metadata"`
	ServiceTier  types.String `tfsdk:"service_tier"`
	Reasoning    types.Object `tfsdk:"reasoning"`
	Text         types.Object `tfsdk:"text"`
	MultiAgent   types.Object `tfsdk:"multi_agent"`
	Tools        types.List   `tfsdk:"tools"`
}

type reasoningModel struct {
	Effort  types.String `tfsdk:"effort"`
	Summary types.String `tfsdk:"summary"`
}

type textModel struct {
	Verbosity types.String `tfsdk:"verbosity"`
	Format    types.Object `tfsdk:"format"`
}

type textFormatModel struct {
	Type       types.String         `tfsdk:"type"`
	SchemaJSON jsontypes.Normalized `tfsdk:"schema_json"`
}

type multiAgentModel struct {
	Enabled                types.Bool  `tfsdk:"enabled"`
	MaxConcurrentSubagents types.Int64 `tfsdk:"max_concurrent_subagents"`
}

func agentWriteFromPlan(ctx context.Context, plan, state agentModel) (client.AgentWrite, diag.Diagnostics) {
	var diags diag.Diagnostics
	write := client.AgentWrite{Model: plan.Model.ValueString()}
	write.Instructions = stringOptional(plan.Instructions, state.Instructions)
	write.Name = stringOptional(plan.Name, state.Name)
	write.ServiceTier = stringOptional(plan.ServiceTier, state.ServiceTier)

	if !plan.Metadata.IsUnknown() {
		if plan.Metadata.IsNull() {
			if !state.Metadata.IsNull() {
				write.Metadata = client.Null[map[string]string]()
			}
		} else {
			m := map[string]string{}
			diags.Append(plan.Metadata.ElementsAs(ctx, &m, false)...)
			write.Metadata = client.Set(m)
		}
	}

	if !plan.Reasoning.IsUnknown() {
		if plan.Reasoning.IsNull() {
			if !state.Reasoning.IsNull() && !state.Reasoning.IsUnknown() {
				write.Reasoning = client.Null[client.Reasoning]()
			}
		} else {
			var rm reasoningModel
			diags.Append(plan.Reasoning.As(ctx, &rm, basetypes.ObjectAsOptions{})...)
			r := client.Reasoning{}
			if !rm.Effort.IsNull() {
				v := rm.Effort.ValueString()
				r.Effort = &v
			}
			if !rm.Summary.IsNull() {
				v := rm.Summary.ValueString()
				r.Summary = &v
			}
			write.Reasoning = client.Set(r)
		}
	}

	if !plan.Text.IsUnknown() {
		if plan.Text.IsNull() {
			if !state.Text.IsNull() && !state.Text.IsUnknown() {
				write.Text = client.Null[client.Text]()
			}
		} else {
			tm, d := textFromObject(ctx, plan.Text)
			diags.Append(d...)
			write.Text = client.Set(tm)
		}
	}

	if !plan.MultiAgent.IsUnknown() {
		if plan.MultiAgent.IsNull() {
			if !state.MultiAgent.IsNull() && !state.MultiAgent.IsUnknown() {
				write.MultiAgent = client.Null[client.MultiAgent]()
			}
		} else {
			var mm multiAgentModel
			diags.Append(plan.MultiAgent.As(ctx, &mm, basetypes.ObjectAsOptions{})...)
			ma := client.MultiAgent{Enabled: mm.Enabled.ValueBool()}
			if !mm.MaxConcurrentSubagents.IsNull() {
				v := mm.MaxConcurrentSubagents.ValueInt64()
				ma.MaxConcurrentSubagents = &v
			}
			write.MultiAgent = client.Set(ma)
		}
	}

	if !plan.Tools.IsUnknown() {
		tools, d := toolsToAPI(ctx, plan.Tools)
		diags.Append(d...)
		write.Tools = client.Set(tools)
	}

	return write, diags
}

func stringOptional(plan, state types.String) client.Optional[string] {
	if !plan.IsUnknown() && !plan.IsNull() {
		return client.Set(plan.ValueString())
	}
	if plan.IsNull() && !state.IsNull() && !state.IsUnknown() {
		return client.Null[string]()
	}
	return client.Optional[string]{}
}

func textFromObject(ctx context.Context, obj types.Object) (client.Text, diag.Diagnostics) {
	var diags diag.Diagnostics
	var tm textModel
	diags.Append(obj.As(ctx, &tm, basetypes.ObjectAsOptions{})...)
	out := client.Text{}
	if !tm.Verbosity.IsNull() {
		v := tm.Verbosity.ValueString()
		out.Verbosity = &v
	}
	if !tm.Format.IsNull() && !tm.Format.IsUnknown() {
		var fm textFormatModel
		diags.Append(tm.Format.As(ctx, &fm, basetypes.ObjectAsOptions{})...)
		format := &client.TextFormat{Type: fm.Type.ValueString()}
		if !fm.SchemaJSON.IsNull() && fm.SchemaJSON.ValueString() != "" {
			canon, err := client.CanonicalJSON(fm.SchemaJSON.ValueString())
			if err != nil {
				diags.AddError("Invalid text format schema_json", err.Error())
			} else {
				format.Schema = json.RawMessage(canon)
			}
		}
		out.Format = format
	}
	return out, diags
}

func agentToState(ctx context.Context, prior agentModel, agent *client.Agent) (agentModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := prior
	out.ID = types.StringValue(agent.ID)
	out.Object = types.StringValue(agent.Object)
	out.CreatedAt = types.Int64Value(agent.CreatedAt)
	out.UpdatedAt = types.Int64Value(agent.UpdatedAt)
	out.Model = types.StringValue(agent.Model)
	out.Instructions = stringPtrValue(agent.Instructions)
	out.Name = stringPtrValue(agent.Name)

	meta := agent.Metadata
	if meta == nil {
		meta = map[string]string{}
	}
	mv, d := types.MapValueFrom(ctx, types.StringType, meta)
	diags.Append(d...)
	out.Metadata = mv

	if prior.ServiceTier.IsNull() || prior.ServiceTier.IsUnknown() {
		out.ServiceTier = types.StringNull()
	} else {
		out.ServiceTier = types.StringValue(agent.ServiceTier)
	}

	if prior.Reasoning.IsNull() || prior.Reasoning.IsUnknown() {
		out.Reasoning = types.ObjectNull(map[string]attr.Type{"effort": types.StringType, "summary": types.StringType})
	} else if agent.Reasoning != nil {
		obj, d := types.ObjectValueFrom(ctx, map[string]attr.Type{"effort": types.StringType, "summary": types.StringType}, reasoningModel{
			Effort:  stringPtrValue(agent.Reasoning.Effort),
			Summary: stringPtrValue(agent.Reasoning.Summary),
		})
		diags.Append(d...)
		out.Reasoning = obj
	}

	if prior.Text.IsNull() || prior.Text.IsUnknown() {
		out.Text = types.ObjectNull(textAttrTypes)
	} else if agent.Text != nil {
		obj, d := textToObject(ctx, *agent.Text)
		diags.Append(d...)
		out.Text = obj
	}

	if prior.MultiAgent.IsNull() || prior.MultiAgent.IsUnknown() {
		out.MultiAgent = types.ObjectNull(multiAgentAttrTypes)
	} else if agent.MultiAgent != nil {
		mm := multiAgentModel{Enabled: types.BoolValue(agent.MultiAgent.Enabled)}
		if agent.MultiAgent.MaxConcurrentSubagents != nil {
			mm.MaxConcurrentSubagents = types.Int64Value(*agent.MultiAgent.MaxConcurrentSubagents)
		} else {
			mm.MaxConcurrentSubagents = types.Int64Null()
		}
		obj, d := types.ObjectValueFrom(ctx, multiAgentAttrTypes, mm)
		diags.Append(d...)
		out.MultiAgent = obj
	}

	tools, d := toolsFromAPI(ctx, agent.Tools)
	diags.Append(d...)
	out.Tools = tools
	return out, diags
}

var reasoningAttrTypes = map[string]attr.Type{"effort": types.StringType, "summary": types.StringType}

var formatAttrTypes = map[string]attr.Type{
	"type":        types.StringType,
	"schema_json": jsontypes.NormalizedType{},
}

var textAttrTypes = map[string]attr.Type{
	"verbosity": types.StringType,
	"format":    types.ObjectType{AttrTypes: formatAttrTypes},
}

var multiAgentAttrTypes = map[string]attr.Type{
	"enabled":                  types.BoolType,
	"max_concurrent_subagents": types.Int64Type,
}

func textToObject(ctx context.Context, text client.Text) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	tm := textModel{Verbosity: stringPtrValue(text.Verbosity), Format: types.ObjectNull(formatAttrTypes)}
	if text.Format != nil {
		fm := textFormatModel{Type: types.StringValue(text.Format.Type), SchemaJSON: jsontypes.NewNormalizedNull()}
		if len(text.Format.Schema) > 0 {
			canon, err := client.CanonicalJSON(string(text.Format.Schema))
			if err != nil {
				diags.AddError("Invalid text format schema from API", err.Error())
			} else {
				fm.SchemaJSON = jsontypes.NewNormalizedValue(canon)
			}
		}
		obj, d := types.ObjectValueFrom(ctx, formatAttrTypes, fm)
		diags.Append(d...)
		tm.Format = obj
	}
	obj, d := types.ObjectValueFrom(ctx, textAttrTypes, tm)
	diags.Append(d...)
	return obj, diags
}

func stringPtrValue(v *string) types.String {
	if v == nil {
		return types.StringNull()
	}
	return types.StringValue(*v)
}

func toolsToAPI(ctx context.Context, list types.List) ([]json.RawMessage, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() {
		return []json.RawMessage{}, nil
	}
	var elems []types.Object
	diags.Append(list.ElementsAs(ctx, &elems, false)...)
	out := make([]json.RawMessage, 0, len(elems))
	for i, elem := range elems {
		raw, d := toolToAPI(ctx, elem)
		diags.Append(d...)
		if d.HasError() {
			diags.AddError("Invalid tool", fmt.Sprintf("tools[%d] could not be encoded", i))
			continue
		}
		out = append(out, raw)
	}
	if err := client.ValidatePersistedTools(out); err != nil {
		diags.AddError("Invalid tools", err.Error())
	}
	return out, diags
}

func toolToAPI(ctx context.Context, obj types.Object) (json.RawMessage, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrs := obj.Attributes()
	typ := attrs["type"].(types.String).ValueString()
	body := map[string]any{"type": typ}
	switch typ {
	case "function":
		if attrs["function"].(types.Object).IsNull() {
			diags.AddError("Invalid function tool", "function block is required when type is function")
			return nil, diags
		}
		var fm struct {
			Name           types.String         `tfsdk:"name"`
			Description    types.String         `tfsdk:"description"`
			ParametersJSON jsontypes.Normalized `tfsdk:"parameters_json"`
			DeferLoading   types.Bool           `tfsdk:"defer_loading"`
		}
		diags.Append(attrs["function"].(types.Object).As(ctx, &fm, basetypes.ObjectAsOptions{})...)
		canon, err := client.CanonicalJSON(fm.ParametersJSON.ValueString())
		if err != nil {
			diags.AddError("Invalid function parameters_json", err.Error())
			return nil, diags
		}
		body["name"] = fm.Name.ValueString()
		body["description"] = fm.Description.ValueString()
		var params any
		_ = json.Unmarshal([]byte(canon), &params)
		body["parameters"] = params
		if !fm.DeferLoading.IsNull() {
			body["defer_loading"] = fm.DeferLoading.ValueBool()
		}
	case "tool_search":
	case "programmatic_tool_calling":
		ptc := attrs["programmatic_tool_calling"].(types.Object)
		if !ptc.IsNull() && !ptc.IsUnknown() {
			var pm struct {
				Enabled types.Bool `tfsdk:"enabled"`
			}
			diags.Append(ptc.As(ctx, &pm, basetypes.ObjectAsOptions{})...)
			if !pm.Enabled.IsNull() {
				body["enabled"] = pm.Enabled.ValueBool()
			}
		}
	case "mcp":
		mcpObj := attrs["mcp"].(types.Object)
		if mcpObj.IsNull() {
			diags.AddError("Invalid mcp tool", "mcp block is required when type is mcp")
			return nil, diags
		}
		mcpBody, d := mcpToAPI(ctx, mcpObj)
		diags.Append(d...)
		for k, v := range mcpBody {
			body[k] = v
		}
	case "web_search":
		ws := attrs["web_search"].(types.Object)
		if !ws.IsNull() && !ws.IsUnknown() {
			wsBody, d := webSearchToAPI(ctx, ws)
			diags.Append(d...)
			for k, v := range wsBody {
				body[k] = v
			}
		}
	default:
		diags.AddError("Unsupported tool type", fmt.Sprintf("unsupported tool type %q; supported: function, tool_search, programmatic_tool_calling, mcp, web_search", typ))
		return nil, diags
	}
	raw, err := json.Marshal(body)
	if err != nil {
		diags.AddError("Invalid tool", err.Error())
	}
	return raw, diags
}

func mcpToAPI(ctx context.Context, obj types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	var m struct {
		ServerLabel         types.String         `tfsdk:"server_label"`
		Transport           types.Object         `tfsdk:"transport"`
		AllowedTools        types.List           `tfsdk:"allowed_tools"`
		ConnectionOrigin    types.String         `tfsdk:"connection_origin"`
		CredentialID        types.String         `tfsdk:"credential_id"`
		RequestMetadataJSON jsontypes.Normalized `tfsdk:"request_metadata_json"`
		Required            types.Bool           `tfsdk:"required"`
	}
	diags.Append(obj.As(ctx, &m, basetypes.ObjectAsOptions{})...)
	body := map[string]any{"server_label": m.ServerLabel.ValueString()}
	transport, d := transportToAPI(ctx, m.Transport)
	diags.Append(d...)
	body["transport"] = transport
	if !m.AllowedTools.IsNull() && !m.AllowedTools.IsUnknown() {
		var tools []string
		diags.Append(m.AllowedTools.ElementsAs(ctx, &tools, false)...)
		if tools == nil {
			tools = []string{}
		}
		body["allowed_tools"] = tools
	}
	if !m.ConnectionOrigin.IsNull() {
		body["connection_origin"] = m.ConnectionOrigin.ValueString()
	}
	if !m.CredentialID.IsNull() {
		body["credential_id"] = m.CredentialID.ValueString()
	}
	if !m.RequestMetadataJSON.IsNull() && m.RequestMetadataJSON.ValueString() != "" {
		var meta any
		canon, err := client.CanonicalJSON(m.RequestMetadataJSON.ValueString())
		if err != nil {
			diags.AddError("Invalid request_metadata_json", err.Error())
		} else {
			_ = json.Unmarshal([]byte(canon), &meta)
			body["request_metadata"] = meta
		}
	}
	if !m.Required.IsNull() {
		body["required"] = m.Required.ValueBool()
	}
	return body, diags
}

func transportToAPI(ctx context.Context, obj types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	var t struct {
		Type      types.String `tfsdk:"type"`
		ServerURL types.String `tfsdk:"server_url"`
		Headers   types.Map    `tfsdk:"headers"`
		Command   types.String `tfsdk:"command"`
		Cwd       types.String `tfsdk:"cwd"`
		Args      types.List   `tfsdk:"args"`
		EnvVars   types.List   `tfsdk:"env_vars"`
	}
	diags.Append(obj.As(ctx, &t, basetypes.ObjectAsOptions{})...)
	body := map[string]any{"type": t.Type.ValueString()}
	if !t.ServerURL.IsNull() {
		body["server_url"] = t.ServerURL.ValueString()
	}
	if !t.Headers.IsNull() && !t.Headers.IsUnknown() {
		headers := map[string]string{}
		diags.Append(t.Headers.ElementsAs(ctx, &headers, false)...)
		body["headers"] = headers
	}
	if !t.Command.IsNull() {
		body["command"] = t.Command.ValueString()
	}
	if !t.Cwd.IsNull() {
		body["cwd"] = t.Cwd.ValueString()
	}
	if !t.Args.IsNull() && !t.Args.IsUnknown() {
		var args []string
		diags.Append(t.Args.ElementsAs(ctx, &args, false)...)
		body["args"] = args
	}
	if !t.EnvVars.IsNull() && !t.EnvVars.IsUnknown() {
		var env []string
		diags.Append(t.EnvVars.ElementsAs(ctx, &env, false)...)
		body["env_vars"] = env
	}
	return body, diags
}

func webSearchToAPI(ctx context.Context, obj types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	var w struct {
		AllowedDomains types.List   `tfsdk:"allowed_domains"`
		ContextSize    types.String `tfsdk:"context_size"`
		Mode           types.String `tfsdk:"mode"`
		Location       types.Object `tfsdk:"location"`
	}
	diags.Append(obj.As(ctx, &w, basetypes.ObjectAsOptions{})...)
	body := map[string]any{}
	if !w.AllowedDomains.IsNull() && !w.AllowedDomains.IsUnknown() {
		var domains []string
		diags.Append(w.AllowedDomains.ElementsAs(ctx, &domains, false)...)
		body["allowed_domains"] = domains
	}
	if !w.ContextSize.IsNull() {
		body["context_size"] = w.ContextSize.ValueString()
	}
	if !w.Mode.IsNull() {
		body["mode"] = w.Mode.ValueString()
	}
	if !w.Location.IsNull() && !w.Location.IsUnknown() {
		var loc struct {
			City     types.String `tfsdk:"city"`
			Country  types.String `tfsdk:"country"`
			Region   types.String `tfsdk:"region"`
			Timezone types.String `tfsdk:"timezone"`
		}
		diags.Append(w.Location.As(ctx, &loc, basetypes.ObjectAsOptions{})...)
		lm := map[string]any{}
		if !loc.City.IsNull() {
			lm["city"] = loc.City.ValueString()
		}
		if !loc.Country.IsNull() {
			lm["country"] = loc.Country.ValueString()
		}
		if !loc.Region.IsNull() {
			lm["region"] = loc.Region.ValueString()
		}
		if !loc.Timezone.IsNull() {
			lm["timezone"] = loc.Timezone.ValueString()
		}
		body["location"] = lm
	}
	return body, diags
}

func toolsFromAPI(ctx context.Context, tools []json.RawMessage) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	if tools == nil {
		tools = []json.RawMessage{}
	}
	elems := make([]attr.Value, 0, len(tools))
	for _, raw := range tools {
		obj, d := toolFromAPI(ctx, raw)
		diags.Append(d...)
		elems = append(elems, obj)
	}
	list, d := types.ListValue(types.ObjectType{AttrTypes: toolAttrTypes}, elems)
	diags.Append(d...)
	return list, diags
}

func toolFromAPI(ctx context.Context, raw json.RawMessage) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		diags.AddError("Invalid tool from API", err.Error())
		return types.ObjectNull(toolAttrTypes), diags
	}
	attrs := map[string]attr.Value{
		"type":                      types.StringValue(probe.Type),
		"function":                  types.ObjectNull(functionAttrTypes),
		"programmatic_tool_calling": types.ObjectNull(ptcAttrTypes),
		"mcp":                       types.ObjectNull(mcpAttrTypes),
		"web_search":                types.ObjectNull(webSearchAttrTypes),
	}
	switch probe.Type {
	case "function":
		obj, d := functionFromAPI(ctx, raw)
		diags.Append(d...)
		attrs["function"] = obj
	case "programmatic_tool_calling":
		obj, d := ptcFromAPI(ctx, raw)
		diags.Append(d...)
		attrs["programmatic_tool_calling"] = obj
	case "mcp":
		obj, d := mcpFromAPI(ctx, raw)
		diags.Append(d...)
		attrs["mcp"] = obj
	case "web_search":
		obj, d := webSearchFromAPI(ctx, raw)
		diags.Append(d...)
		attrs["web_search"] = obj
	case "tool_search":
	default:
		diags.AddError("Unsupported tool type", fmt.Sprintf("unsupported tool type %q returned by API", probe.Type))
	}
	obj, d := types.ObjectValue(toolAttrTypes, attrs)
	diags.Append(d...)
	return obj, diags
}

func functionFromAPI(ctx context.Context, raw json.RawMessage) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	var api struct {
		Name         string          `json:"name"`
		Description  string          `json:"description"`
		Parameters   json.RawMessage `json:"parameters"`
		DeferLoading *bool           `json:"defer_loading"`
	}
	if err := json.Unmarshal(raw, &api); err != nil {
		diags.AddError("Invalid function tool", err.Error())
		return types.ObjectNull(functionAttrTypes), diags
	}
	canon, err := client.CanonicalJSON(string(api.Parameters))
	if err != nil {
		diags.AddError("Invalid function parameters", err.Error())
		canon = "{}"
	}
	fm := map[string]attr.Value{
		"name":            types.StringValue(api.Name),
		"description":     types.StringValue(api.Description),
		"parameters_json": jsontypes.NewNormalizedValue(canon),
		"defer_loading":   types.BoolNull(),
	}
	if api.DeferLoading != nil {
		fm["defer_loading"] = types.BoolValue(*api.DeferLoading)
	}
	obj, d := types.ObjectValue(functionAttrTypes, fm)
	diags.Append(d...)
	return obj, diags
}

func ptcFromAPI(_ context.Context, raw json.RawMessage) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	var api struct {
		Enabled *bool `json:"enabled"`
	}
	_ = json.Unmarshal(raw, &api)
	vals := map[string]attr.Value{"enabled": types.BoolNull()}
	if api.Enabled != nil {
		vals["enabled"] = types.BoolValue(*api.Enabled)
	}
	obj, d := types.ObjectValue(ptcAttrTypes, vals)
	diags.Append(d...)
	return obj, diags
}

func mcpFromAPI(ctx context.Context, raw json.RawMessage) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	var api struct {
		ServerLabel      string          `json:"server_label"`
		Transport        json.RawMessage `json:"transport"`
		AllowedTools     json.RawMessage `json:"allowed_tools"`
		ConnectionOrigin *string         `json:"connection_origin"`
		CredentialID     *string         `json:"credential_id"`
		RequestMetadata  json.RawMessage `json:"request_metadata"`
		Required         *bool           `json:"required"`
	}
	if err := json.Unmarshal(raw, &api); err != nil {
		diags.AddError("Invalid mcp tool", err.Error())
		return types.ObjectNull(mcpAttrTypes), diags
	}
	transport, d := transportFromAPI(ctx, api.Transport)
	diags.Append(d...)
	allowed := types.ListNull(types.StringType)
	if len(api.AllowedTools) > 0 && string(api.AllowedTools) != "null" {
		var tools []string
		_ = json.Unmarshal(api.AllowedTools, &tools)
		if tools == nil {
			tools = []string{}
		}
		lv, d := types.ListValueFrom(ctx, types.StringType, tools)
		diags.Append(d...)
		allowed = lv
	}
	reqMeta := jsontypes.NewNormalizedNull()
	if len(api.RequestMetadata) > 0 && string(api.RequestMetadata) != "null" {
		canon, err := client.CanonicalJSON(string(api.RequestMetadata))
		if err == nil {
			reqMeta = jsontypes.NewNormalizedValue(canon)
		}
	}
	vals := map[string]attr.Value{
		"server_label":          types.StringValue(api.ServerLabel),
		"transport":             transport,
		"allowed_tools":         allowed,
		"connection_origin":     stringPtrValue(api.ConnectionOrigin),
		"credential_id":         stringPtrValue(api.CredentialID),
		"request_metadata_json": reqMeta,
		"required":              types.BoolNull(),
	}
	if api.Required != nil {
		vals["required"] = types.BoolValue(*api.Required)
	}
	obj, d := types.ObjectValue(mcpAttrTypes, vals)
	diags.Append(d...)
	return obj, diags
}

func transportFromAPI(ctx context.Context, raw json.RawMessage) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	var api struct {
		Type      string            `json:"type"`
		ServerURL string            `json:"server_url"`
		Headers   map[string]string `json:"headers"`
		Command   string            `json:"command"`
		Cwd       string            `json:"cwd"`
		Args      []string          `json:"args"`
		EnvVars   []string          `json:"env_vars"`
	}
	_ = json.Unmarshal(raw, &api)
	headers := types.MapNull(types.StringType)
	if api.Headers != nil {
		mv, d := types.MapValueFrom(ctx, types.StringType, api.Headers)
		diags.Append(d...)
		headers = mv
	}
	args := types.ListNull(types.StringType)
	if api.Args != nil {
		lv, d := types.ListValueFrom(ctx, types.StringType, api.Args)
		diags.Append(d...)
		args = lv
	}
	env := types.ListNull(types.StringType)
	if api.EnvVars != nil {
		lv, d := types.ListValueFrom(ctx, types.StringType, api.EnvVars)
		diags.Append(d...)
		env = lv
	}
	vals := map[string]attr.Value{
		"type":       types.StringValue(api.Type),
		"server_url": nullIfEmpty(api.ServerURL),
		"headers":    headers,
		"command":    nullIfEmpty(api.Command),
		"cwd":        nullIfEmpty(api.Cwd),
		"args":       args,
		"env_vars":   env,
	}
	obj, d := types.ObjectValue(transportAttrTypes, vals)
	diags.Append(d...)
	return obj, diags
}

func webSearchFromAPI(ctx context.Context, raw json.RawMessage) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	var api struct {
		AllowedDomains []string `json:"allowed_domains"`
		ContextSize    *string  `json:"context_size"`
		Mode           *string  `json:"mode"`
		Location       *struct {
			City     *string `json:"city"`
			Country  *string `json:"country"`
			Region   *string `json:"region"`
			Timezone *string `json:"timezone"`
		} `json:"location"`
	}
	_ = json.Unmarshal(raw, &api)
	domains := types.ListNull(types.StringType)
	if api.AllowedDomains != nil {
		lv, d := types.ListValueFrom(ctx, types.StringType, api.AllowedDomains)
		diags.Append(d...)
		domains = lv
	}
	loc := types.ObjectNull(locationAttrTypes)
	if api.Location != nil {
		obj, d := types.ObjectValue(locationAttrTypes, map[string]attr.Value{
			"city":     stringPtrValue(api.Location.City),
			"country":  stringPtrValue(api.Location.Country),
			"region":   stringPtrValue(api.Location.Region),
			"timezone": stringPtrValue(api.Location.Timezone),
		})
		diags.Append(d...)
		loc = obj
	}
	obj, d := types.ObjectValue(webSearchAttrTypes, map[string]attr.Value{
		"allowed_domains": domains,
		"context_size":    stringPtrValue(api.ContextSize),
		"mode":            stringPtrValue(api.Mode),
		"location":        loc,
	})
	diags.Append(d...)
	return obj, diags
}

func nullIfEmpty(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
