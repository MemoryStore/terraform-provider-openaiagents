// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// typedNestedObjectModifier fills Optional+Computed nested tool objects only
// when tools[i].type matches. UseStateForUnknown on these attributes is
// unsafe: list indexes are not identities, so a reorder would copy the
// previous element's nested object into the new type.
type typedNestedObjectModifier struct {
	toolType   string
	attrTypes  map[string]attr.Type
	defaultObj types.Object
}

func (m typedNestedObjectModifier) Description(context.Context) string {
	return "computes the nested tool object only when the sibling type matches"
}

func (m typedNestedObjectModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m typedNestedObjectModifier) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.ConfigValue.IsUnknown() {
		return
	}
	var typ types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, req.Path.ParentPath().AtName("type"), &typ)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if typ.IsUnknown() {
		resp.PlanValue = types.ObjectUnknown(m.attrTypes)
		return
	}
	if typ.IsNull() || typ.ValueString() != m.toolType {
		resp.PlanValue = types.ObjectNull(m.attrTypes)
		return
	}
	if !req.ConfigValue.IsNull() {
		return
	}
	if !req.StateValue.IsNull() && !req.StateValue.IsUnknown() {
		resp.PlanValue = req.StateValue
		return
	}
	resp.PlanValue = m.defaultObj
}

type maxConcurrentModifier struct{}

func (m maxConcurrentModifier) Description(context.Context) string {
	return "defaults max_concurrent_subagents to 6 only when multi_agent is enabled"
}

func (m maxConcurrentModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m maxConcurrentModifier) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	var enabled types.Bool
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, req.Path.ParentPath().AtName("enabled"), &enabled)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if enabled.IsUnknown() {
		resp.PlanValue = types.Int64Unknown()
		return
	}
	if enabled.IsNull() || !enabled.ValueBool() {
		resp.PlanValue = types.Int64Null()
		return
	}
	if req.ConfigValue.IsUnknown() {
		return
	}
	if !req.ConfigValue.IsNull() {
		return
	}
	resp.PlanValue = types.Int64Value(6)
}

func maxConcurrentPlanModifier() planmodifier.Int64 {
	return maxConcurrentModifier{}
}

func ptcPlanModifier() planmodifier.Object {
	return typedNestedObjectModifier{
		toolType:  "programmatic_tool_calling",
		attrTypes: ptcAttrTypes,
		defaultObj: types.ObjectValueMust(ptcAttrTypes, map[string]attr.Value{
			"enabled": types.BoolValue(true),
		}),
	}
}

func webSearchPlanModifier() planmodifier.Object {
	// Hosted Agents API defaults observed 2026-09-11 against api.openai.com
	// with OpenAI-Beta: agents=v1. A bare {"type":"web_search"} create returned
	// context_size=medium and mode=live. If OpenAI changes those server
	// defaults, a second plan against production fails with inconsistent
	// result until these values are updated.
	return typedNestedObjectModifier{
		toolType:  "web_search",
		attrTypes: webSearchAttrTypes,
		defaultObj: types.ObjectValueMust(webSearchAttrTypes, map[string]attr.Value{
			"allowed_domains": types.ListNull(types.StringType),
			"context_size":    types.StringValue("medium"),
			"mode":            types.StringValue("live"),
			"location":        types.ObjectNull(locationAttrTypes),
		}),
	}
}
