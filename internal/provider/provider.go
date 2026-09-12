// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/MemoryStore/terraform-provider-openaiagents/internal/client"
)

var _ provider.Provider = &OpenAIAgentsProvider{}

// OpenAIAgentsProvider implements the MemoryStore/openaiagents provider.
type OpenAIAgentsProvider struct {
	version         string
	allowProduction bool
}

// OpenAIAgentsProviderModel is the provider configuration.
type OpenAIAgentsProviderModel struct {
	APIKey       types.String `tfsdk:"api_key"`
	Organization types.String `tfsdk:"organization"`
	Project      types.String `tfsdk:"project"`
	BaseURL      types.String `tfsdk:"base_url"`
}

func (p *OpenAIAgentsProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "openaiagents"
	resp.Version = p.version
}

func (p *OpenAIAgentsProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage hosted OpenAI Agents API deployment objects: saved agents, environment templates, and vaults. " +
			"This provider does not create sessions, send messages, or run turns. " +
			"It uses a project API key (`api_key` or `OPENAI_API_KEY`) and never reads `OPENAI_ADMIN_KEY`. " +
			"Terraform >= 1.11 is required for write-only arguments. " +
			"The Registry source is `MemoryStore/openaiagents`.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "OpenAI project API key. If omitted, `OPENAI_API_KEY` is used. " +
					"Configuring the provider does not require the key: a root that declares this provider but enables no objects, " +
					"for example because every resource is held at `count = 0` or in a module with an empty `for_each`, can be planned without one. " +
					"A missing key is reported as a `Missing API key` error by the first operation that calls the API, not during provider configuration. " +
					"That is plan time for a data source or an existing resource being refreshed, and apply time for a resource being created, " +
					"so a plan that only adds new resources can still succeed without a key. " +
					"This provider never reads `OPENAI_ADMIN_KEY`.",
			},
			"organization": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional OpenAI organization ID sent as `OpenAI-Organization`. If omitted, `OPENAI_ORG_ID` or `OPENAI_ORGANIZATION` is used.",
			},
			"project": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional OpenAI project ID sent as `OpenAI-Project`. If omitted, `OPENAI_PROJECT` is used.",
			},
			"base_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "API base URL including the `/v1` prefix. Defaults to `https://api.openai.com/v1`. HTTP is allowed only for localhost or loopback. If omitted, `OPENAI_BASE_URL` is used.",
			},
		},
	}
}

func (p *OpenAIAgentsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data OpenAIAgentsProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.APIKey.IsUnknown() || data.Organization.IsUnknown() || data.Project.IsUnknown() || data.BaseURL.IsUnknown() {
		return
	}

	// An empty key is not an error here. Terraform runs Configure whenever any
	// resource in the graph references the provider, including resources held
	// at count = 0 and modules with an empty for_each, so failing here would
	// force a key on roots that enable nothing. The client reports the same
	// "Missing API key" diagnostic at the first request that needs it.
	apiKey := resolveExplicitOrEnv(data.APIKey, clientAPIKeyEnv)

	organization := resolveExplicitOrEnv(data.Organization, "OPENAI_ORG_ID")
	if organization == "" {
		organization = resolveExplicitOrEnv(data.Organization, "OPENAI_ORGANIZATION")
	}
	project := resolveExplicitOrEnv(data.Project, "OPENAI_PROJECT")
	baseURL := resolveExplicitOrEnv(data.BaseURL, "OPENAI_BASE_URL")

	c, err := client.New(client.Options{
		APIKey:          apiKey,
		Organization:    organization,
		Project:         project,
		BaseURL:         baseURL,
		AllowProduction: p.allowProduction,
		UserAgent:       "terraform-provider-openaiagents/" + p.version,
	})
	if err != nil {
		resp.Diagnostics.AddError("Invalid provider configuration", err.Error())
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *OpenAIAgentsProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAgentResource,
		NewEnvironmentTemplateResource,
		NewVaultResource,
		NewVaultCredentialResource,
	}
}

func (p *OpenAIAgentsProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewAgentDataSource,
		NewEnvironmentTemplateDataSource,
		NewVaultDataSource,
		NewVaultCredentialDataSource,
	}
}

// New returns a provider factory. Production API access is allowed for real runs.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &OpenAIAgentsProvider{
			version:         version,
			allowProduction: true,
		}
	}
}

func newTestProvider(version string) func() provider.Provider {
	return func() provider.Provider {
		return &OpenAIAgentsProvider{
			version:         version,
			allowProduction: false,
		}
	}
}

const clientAPIKeyEnv = "OPENAI_API_KEY"

func resolveExplicitOrEnv(value types.String, envName string) string {
	if value.IsUnknown() {
		return ""
	}
	if !value.IsNull() {
		return value.ValueString()
	}
	return os.Getenv(envName)
}
