// Package provider implements the Brockley Terraform provider.
package provider

import (
	"context"

	"github.com/brockleyai/brockleyai/cli/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = (*BrockleyProvider)(nil)

// BrockleyProvider is the Terraform provider implementation.
type BrockleyProvider struct {
	version string
}

// BrockleyProviderModel describes the provider config.
type BrockleyProviderModel struct {
	ServerURL types.String `tfsdk:"server_url"`
	APIKey    types.String `tfsdk:"api_key"`
}

// New creates a new provider instance.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &BrockleyProvider{version: version}
	}
}

func (p *BrockleyProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "brockley"
	resp.Version = p.version
}

func (p *BrockleyProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for Brockley AI agent infrastructure platform.",
		Attributes: map[string]schema.Attribute{
			"server_url": schema.StringAttribute{
				Description: "Brockley server URL. Can also be set via BROCKLEY_SERVER_URL env var.",
				Optional:    true,
			},
			"api_key": schema.StringAttribute{
				Description: "API key for authentication. Can also be set via BROCKLEY_API_KEY env var.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (p *BrockleyProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config BrockleyProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serverURL := "http://localhost:8000"
	if !config.ServerURL.IsNull() && !config.ServerURL.IsUnknown() {
		serverURL = config.ServerURL.ValueString()
	}

	apiKey := ""
	if !config.APIKey.IsNull() && !config.APIKey.IsUnknown() {
		apiKey = config.APIKey.ValueString()
	}

	c := client.New(serverURL, apiKey)
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *BrockleyProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewGraphResource,
		NewSchemaResource,
		NewPromptTemplateResource,
		NewProviderConfigResource,
		NewAPIToolResource,
	}
}

func (p *BrockleyProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewGraphDataSource,
		NewSchemaDataSource,
		NewPromptTemplateDataSource,
		NewProviderConfigDataSource,
		NewAPIToolDataSource,
	}
}
