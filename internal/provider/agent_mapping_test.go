// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestToolsToAPISkipsUnknownFunctionSchema(t *testing.T) {
	ctx := context.Background()
	fn, diags := types.ObjectValue(functionAttrTypes, map[string]attr.Value{
		"name":            types.StringValue("lookup"),
		"description":     types.StringValue("Look up a record"),
		"parameters_json": jsontypes.NewNormalizedUnknown(),
		"defer_loading":   types.BoolValue(false),
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	tool, diags := types.ObjectValue(toolAttrTypes, map[string]attr.Value{
		"type":                      types.StringValue("function"),
		"function":                  fn,
		"programmatic_tool_calling": types.ObjectNull(ptcAttrTypes),
		"mcp":                       types.ObjectNull(mcpAttrTypes),
		"web_search":                types.ObjectNull(webSearchAttrTypes),
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	list, diags := types.ListValue(types.ObjectType{AttrTypes: toolAttrTypes}, []attr.Value{tool})
	if diags.HasError() {
		t.Fatal(diags)
	}
	raw, diags := toolsToAPI(ctx, list)
	if diags.HasError() {
		t.Fatalf("unknown function schema must wait until apply, got %v", diags)
	}
	if len(raw) != 0 {
		t.Fatalf("expected no encoded tools while schema is unknown, got %d", len(raw))
	}
}

func TestToolsToAPIRejectsConflictingVariant(t *testing.T) {
	ctx := context.Background()
	fn, diags := types.ObjectValue(functionAttrTypes, map[string]attr.Value{
		"name":            types.StringValue("lookup"),
		"description":     types.StringValue("Look up"),
		"parameters_json": jsontypes.NewNormalizedValue(`{"type":"object"}`),
		"defer_loading":   types.BoolValue(false),
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	mcp, diags := types.ObjectValue(mcpAttrTypes, map[string]attr.Value{
		"server_label":          types.StringValue("docs"),
		"transport":             types.ObjectNull(transportAttrTypes),
		"allowed_tools":         types.ListNull(types.StringType),
		"connection_origin":     types.StringNull(),
		"credential_id":         types.StringNull(),
		"request_metadata_json": jsontypes.NewNormalizedNull(),
		"required":              types.BoolValue(false),
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	tool, diags := types.ObjectValue(toolAttrTypes, map[string]attr.Value{
		"type":                      types.StringValue("function"),
		"function":                  fn,
		"programmatic_tool_calling": types.ObjectNull(ptcAttrTypes),
		"mcp":                       mcp,
		"web_search":                types.ObjectNull(webSearchAttrTypes),
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	list, diags := types.ListValue(types.ObjectType{AttrTypes: toolAttrTypes}, []attr.Value{tool})
	if diags.HasError() {
		t.Fatal(diags)
	}
	_, diags = toolsToAPI(ctx, list)
	if !diags.HasError() {
		t.Fatal("expected conflicting tool block error")
	}
	if !strings.Contains(diags.Errors()[0].Detail(), "must not set mcp") && !strings.Contains(diags.Errors()[0].Summary(), "Conflicting") {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestToolsRoundTripAdditionalPropertiesFalse(t *testing.T) {
	ctx := context.Background()
	params := `{"additionalProperties":false,"properties":{"id":{"type":"string"}},"required":["id"],"type":"object"}`
	fn, diags := types.ObjectValue(functionAttrTypes, map[string]attr.Value{
		"name":            types.StringValue("lookup"),
		"description":     types.StringValue("Look up a record"),
		"parameters_json": jsontypes.NewNormalizedValue(params),
		"defer_loading":   types.BoolValue(false),
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	tool, diags := types.ObjectValue(toolAttrTypes, map[string]attr.Value{
		"type":                      types.StringValue("function"),
		"function":                  fn,
		"programmatic_tool_calling": types.ObjectNull(ptcAttrTypes),
		"mcp":                       types.ObjectNull(mcpAttrTypes),
		"web_search":                types.ObjectNull(webSearchAttrTypes),
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	list, diags := types.ListValue(types.ObjectType{AttrTypes: toolAttrTypes}, []attr.Value{tool})
	if diags.HasError() {
		t.Fatal(diags)
	}
	raw, diags := toolsToAPI(ctx, list)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if len(raw) != 1 {
		t.Fatalf("len=%d", len(raw))
	}
	if !strings.Contains(string(raw[0]), `"additionalProperties":false`) {
		t.Fatalf("lost boolean false: %s", raw[0])
	}
	back, diags := toolsFromAPI(ctx, raw)
	if diags.HasError() {
		t.Fatal(diags)
	}
	again, diags := toolsToAPI(ctx, back)
	if diags.HasError() {
		t.Fatal(diags)
	}
	var a, b any
	_ = json.Unmarshal(raw[0], &a)
	_ = json.Unmarshal(again[0], &b)
	as, _ := json.Marshal(a)
	bs, _ := json.Marshal(b)
	if string(as) != string(bs) {
		t.Fatalf("round trip changed tool:\n%s\n%s", as, bs)
	}
}

func TestAgentWriteClearsRemovedFields(t *testing.T) {
	ctx := context.Background()
	plan := agentModel{
		Model:        types.StringValue("gpt-6-astra"),
		Instructions: types.StringNull(),
		Name:         types.StringNull(),
		Metadata:     types.MapValueMust(types.StringType, map[string]attr.Value{}),
		Tools:        types.ListValueMust(types.ObjectType{AttrTypes: toolAttrTypes}, []attr.Value{}),
	}
	state := agentModel{
		Model:        types.StringValue("gpt-6-astra"),
		Instructions: types.StringValue("old"),
		Name:         types.StringValue("old"),
	}
	write, diags := agentWriteFromPlan(ctx, plan, state)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !write.Instructions.Present || !write.Instructions.Null {
		t.Fatalf("expected instructions null, got %+v", write.Instructions)
	}
	if !write.Name.Present || !write.Name.Null {
		t.Fatalf("expected name null, got %+v", write.Name)
	}
}
