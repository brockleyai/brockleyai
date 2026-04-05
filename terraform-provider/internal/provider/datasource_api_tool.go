package provider

import (
	"context"
	"encoding/json"

	"github.com/brockleyai/brockleyai/cli/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// APIToolDataSource reads a brockley API tool definition.
type APIToolDataSource struct {
	client *client.Client
}

var _ datasource.DataSource = (*APIToolDataSource)(nil)

func NewAPIToolDataSource() datasource.DataSource { return &APIToolDataSource{} }

func (d *APIToolDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_tool"
}

func (d *APIToolDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a Brockley API tool definition.",
		Attributes: map[string]schema.Attribute{
			"id":                 schema.StringAttribute{Required: true},
			"name":               schema.StringAttribute{Computed: true},
			"namespace":          schema.StringAttribute{Computed: true},
			"description":        schema.StringAttribute{Computed: true},
			"base_url":           schema.StringAttribute{Computed: true},
			"default_timeout_ms": schema.Int64Attribute{Computed: true},
			"default_headers":    schema.StringAttribute{Computed: true},
			"retry":              schema.StringAttribute{Computed: true},
			"endpoints": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"metadata": schema.StringAttribute{Computed: true},
		},
	}
}

func (d *APIToolDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected type", "Expected *client.Client")
		return
	}
	d.client = c
}

type apiToolDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Namespace        types.String `tfsdk:"namespace"`
	Description      types.String `tfsdk:"description"`
	BaseURL          types.String `tfsdk:"base_url"`
	DefaultTimeoutMs types.Int64  `tfsdk:"default_timeout_ms"`
	DefaultHeaders   types.String `tfsdk:"default_headers"`
	Retry            types.String `tfsdk:"retry"`
	Endpoints        types.String `tfsdk:"endpoints"`
	Metadata         types.String `tfsdk:"metadata"`
}

func (d *APIToolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg apiToolDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := d.client.GetAPITool(ctx, cfg.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read API tool", err.Error())
		return
	}

	var m map[string]any
	json.Unmarshal(result, &m)
	if v, ok := m["name"].(string); ok {
		cfg.Name = types.StringValue(v)
	}
	if v, ok := m["namespace"].(string); ok {
		cfg.Namespace = types.StringValue(v)
	}
	if v, ok := m["description"].(string); ok {
		cfg.Description = types.StringValue(v)
	}
	if v, ok := m["base_url"].(string); ok {
		cfg.BaseURL = types.StringValue(v)
	}
	if v, ok := m["default_timeout_ms"].(float64); ok {
		cfg.DefaultTimeoutMs = types.Int64Value(int64(v))
	}
	if v, ok := m["default_headers"]; ok {
		b, _ := json.Marshal(v)
		cfg.DefaultHeaders = types.StringValue(string(b))
	}
	if v, ok := m["retry"]; ok {
		b, _ := json.Marshal(v)
		cfg.Retry = types.StringValue(string(b))
	}
	if v, ok := m["endpoints"]; ok {
		b, _ := json.Marshal(v)
		cfg.Endpoints = types.StringValue(string(b))
	}
	if v, ok := m["metadata"]; ok {
		b, _ := json.Marshal(v)
		cfg.Metadata = types.StringValue(string(b))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
