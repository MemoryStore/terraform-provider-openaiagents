// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var locationAttrTypes = map[string]attr.Type{
	"city":     types.StringType,
	"country":  types.StringType,
	"region":   types.StringType,
	"timezone": types.StringType,
}

var functionAttrTypes = map[string]attr.Type{
	"name":            types.StringType,
	"description":     types.StringType,
	"parameters_json": jsontypes.NormalizedType{},
	"defer_loading":   types.BoolType,
}

var ptcAttrTypes = map[string]attr.Type{
	"enabled": types.BoolType,
}

var transportAttrTypes = map[string]attr.Type{
	"type":       types.StringType,
	"server_url": types.StringType,
	"headers":    types.MapType{ElemType: types.StringType},
	"command":    types.StringType,
	"cwd":        types.StringType,
	"args":       types.ListType{ElemType: types.StringType},
	"env_vars":   types.ListType{ElemType: types.StringType},
}

var mcpAttrTypes = map[string]attr.Type{
	"server_label":          types.StringType,
	"transport":             types.ObjectType{AttrTypes: transportAttrTypes},
	"allowed_tools":         types.ListType{ElemType: types.StringType},
	"connection_origin":     types.StringType,
	"credential_id":         types.StringType,
	"request_metadata_json": jsontypes.NormalizedType{},
	"required":              types.BoolType,
}

var webSearchAttrTypes = map[string]attr.Type{
	"allowed_domains": types.ListType{ElemType: types.StringType},
	"context_size":    types.StringType,
	"mode":            types.StringType,
	"location":        types.ObjectType{AttrTypes: locationAttrTypes},
}

var toolAttrTypes = map[string]attr.Type{
	"type":                      types.StringType,
	"function":                  types.ObjectType{AttrTypes: functionAttrTypes},
	"programmatic_tool_calling": types.ObjectType{AttrTypes: ptcAttrTypes},
	"mcp":                       types.ObjectType{AttrTypes: mcpAttrTypes},
	"web_search":                types.ObjectType{AttrTypes: webSearchAttrTypes},
}

func agentResourceSchema() schema.Schema {
	return schema.Schema{
		MarkdownDescription: "A reusable saved agent in the hosted OpenAI Agents API (`POST /v1/agents`). " +
			"Credentials are not stored on the agent; MCP authentication uses vault credential references. " +
			"This resource never creates sessions or runs turns.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Remote agent ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"object": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Object type. Always `agent`.",
			},
			"created_at": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Unix timestamp when the agent was created.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Unix timestamp when the agent was last updated.",
			},
			"model": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Model name as supplied. The requested name is preserved; this provider does not rewrite it.",
			},
			"instructions": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Custom instructions appended to the agent default. Removing this attribute sends JSON null so the API clears it.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Human-readable name. Removing this attribute sends JSON null so the API clears it.",
			},
			"metadata": schema.MapAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{})),
				MarkdownDescription: "Up to 16 string key-value pairs. Removing keys or the map sends an empty object so the API clears them.",
			},
			"service_tier": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Service tier: `auto`, `default`, `flex`, `priority`, or `fast`. Omitted on create so the API default applies.",
			},
			"reasoning": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Reasoning configuration. Removing the block sends JSON null.",
				Attributes: map[string]schema.Attribute{
					"effort": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Reasoning effort: `none`, `minimal`, `low`, `medium`, `high`, `xhigh`, or `max`.",
					},
					"summary": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Reasoning summary: `concise`, `detailed`, or `auto`.",
					},
				},
			},
			"text": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Text output configuration. Removing the block sends JSON null.",
				Attributes: map[string]schema.Attribute{
					"verbosity": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Text verbosity: `low`, `medium`, or `high`.",
					},
					"format": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Output format.",
						Attributes: map[string]schema.Attribute{
							"type": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "`text` or `json_schema`.",
							},
							"schema_json": schema.StringAttribute{
								Optional:            true,
								CustomType:          jsontypes.NormalizedType{},
								MarkdownDescription: "JSON Schema object as a JSON string. Required when `type` is `json_schema`. Nested keywords and booleans such as `additionalProperties: false` are preserved.",
							},
						},
					},
				},
			},
			"multi_agent": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Subagent configuration. Removing the block sends JSON null so multi-agent behavior is disabled.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Required:            true,
						MarkdownDescription: "Whether subagent tools are enabled.",
					},
					"max_concurrent_subagents": schema.Int64Attribute{
						Optional:            true,
						MarkdownDescription: "Maximum concurrent subagents. Defaults to 6 when enabled.",
					},
				},
			},
			"tools": schema.ListNestedAttribute{
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListValueMust(types.ObjectType{AttrTypes: toolAttrTypes}, []attr.Value{})),
				MarkdownDescription: "Persisted tool union. Supported types: `function`, `tool_search`, `programmatic_tool_calling`, `mcp`, `web_search`. Unsupported types are rejected. MCP inline authorization is rejected.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: toolSchemaAttributes(),
				},
			},
		},
	}
}

func toolSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"type": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "Tool type.",
		},
		"function": schema.SingleNestedAttribute{
			Optional:            true,
			MarkdownDescription: "Function tool. Required when `type` is `function`.",
			Attributes: map[string]schema.Attribute{
				"name":            schema.StringAttribute{Required: true, MarkdownDescription: "Function name."},
				"description":     schema.StringAttribute{Required: true, MarkdownDescription: "Function description."},
				"parameters_json": schema.StringAttribute{Required: true, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "JSON Schema for function arguments as a JSON string."},
				"defer_loading":   schema.BoolAttribute{Optional: true, MarkdownDescription: "Whether the function is deferred and discovered through tool search."},
			},
		},
		"programmatic_tool_calling": schema.SingleNestedAttribute{
			Optional:            true,
			MarkdownDescription: "Programmatic tool calling. Used when `type` is `programmatic_tool_calling`.",
			Attributes: map[string]schema.Attribute{
				"enabled": schema.BoolAttribute{Optional: true, MarkdownDescription: "Whether tools can be called from model-generated code. Defaults to true."},
			},
		},
		"mcp": schema.SingleNestedAttribute{
			Optional:            true,
			MarkdownDescription: "Credential-free MCP server. Required when `type` is `mcp`.",
			Attributes: map[string]schema.Attribute{
				"server_label": schema.StringAttribute{Required: true, MarkdownDescription: "Label used to identify the MCP server in tool calls."},
				"transport": schema.SingleNestedAttribute{
					Required:            true,
					MarkdownDescription: "Credential-free transport. HTTP `authorization` and credential-bearing headers are rejected.",
					Attributes: map[string]schema.Attribute{
						"type":       schema.StringAttribute{Required: true, MarkdownDescription: "`http` or `stdio`."},
						"server_url": schema.StringAttribute{Optional: true, MarkdownDescription: "MCP server URL. Required for `http`."},
						"headers":    schema.MapAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Non-secret HTTP headers."},
						"command":    schema.StringAttribute{Optional: true, MarkdownDescription: "Command used to start a stdio MCP server."},
						"cwd":        schema.StringAttribute{Optional: true, MarkdownDescription: "Working directory for a stdio MCP server."},
						"args":       schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Arguments passed to a stdio MCP server command."},
						"env_vars":   schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Environment variable names inherited from the execution environment."},
					},
				},
				"allowed_tools":         schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "MCP tools the agent may call. Omitted means all tools are allowed; an empty list allows none."},
				"connection_origin":     schema.StringAttribute{Optional: true, MarkdownDescription: "`service` (OpenAI network) or `environment` (session environment)."},
				"credential_id":         schema.StringAttribute{Optional: true, MarkdownDescription: "Vault credential selected for this MCP server."},
				"request_metadata_json": schema.StringAttribute{Optional: true, CustomType: jsontypes.NormalizedType{}, MarkdownDescription: "Metadata included with requests to this MCP server, as a JSON object string."},
				"required":              schema.BoolAttribute{Optional: true, MarkdownDescription: "Whether this MCP server must initialize before the first turn."},
			},
		},
		"web_search": schema.SingleNestedAttribute{
			Optional:            true,
			MarkdownDescription: "Web search tool. Used when `type` is `web_search`.",
			Attributes: map[string]schema.Attribute{
				"allowed_domains": schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Domains the search may include."},
				"context_size":    schema.StringAttribute{Optional: true, MarkdownDescription: "`low`, `medium`, or `high`."},
				"mode":            schema.StringAttribute{Optional: true, MarkdownDescription: "`disabled`, `cached`, or `live`."},
				"location": schema.SingleNestedAttribute{
					Optional:            true,
					MarkdownDescription: "Approximate user location.",
					Attributes: map[string]schema.Attribute{
						"city":     schema.StringAttribute{Optional: true, MarkdownDescription: "City name."},
						"country":  schema.StringAttribute{Optional: true, MarkdownDescription: "Two-letter ISO country code."},
						"region":   schema.StringAttribute{Optional: true, MarkdownDescription: "Region or state name."},
						"timezone": schema.StringAttribute{Optional: true, MarkdownDescription: "IANA timezone."},
					},
				},
			},
		},
	}
}
