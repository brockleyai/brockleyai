package provider

import (
	"context"
	"encoding/json"

	"github.com/brockleyai/brockleyai/cli/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// GraphDataSource reads a brockley graph.
type GraphDataSource struct {
	client *client.Client
}

var _ datasource.DataSource = (*GraphDataSource)(nil)

func NewGraphDataSource() datasource.DataSource { return &GraphDataSource{} }

func (d *GraphDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_graph"
}

func (d *GraphDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a Brockley graph.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Required: true},
			"name":        schema.StringAttribute{Computed: true},
			"namespace":   schema.StringAttribute{Computed: true},
			"description": schema.StringAttribute{Computed: true},
			"status":      schema.StringAttribute{Computed: true},
			"version":     schema.Int64Attribute{Computed: true},
			"nodes":       schema.StringAttribute{Computed: true},
			"edges":       schema.StringAttribute{Computed: true},
		},
	}
}

func (d *GraphDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

type graphDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Namespace   types.String `tfsdk:"namespace"`
	Description types.String `tfsdk:"description"`
	Status      types.String `tfsdk:"status"`
	Version     types.Int64  `tfsdk:"version"`
	Nodes       types.String `tfsdk:"nodes"`
	Edges       types.String `tfsdk:"edges"`
}

func (d *GraphDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg graphDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := d.client.GetGraph(ctx, cfg.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read graph", err.Error())
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
	if v, ok := m["status"].(string); ok {
		cfg.Status = types.StringValue(v)
	}
	if v, ok := m["version"].(float64); ok {
		cfg.Version = types.Int64Value(int64(v))
	}
	if v, ok := m["nodes"]; ok {
		b, _ := json.Marshal(v)
		cfg.Nodes = types.StringValue(string(b))
	}
	if v, ok := m["edges"]; ok {
		b, _ := json.Marshal(v)
		cfg.Edges = types.StringValue(string(b))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// SchemaDataSource reads a brockley schema.
type SchemaDataSource struct {
	client *client.Client
}

var _ datasource.DataSource = (*SchemaDataSource)(nil)

func NewSchemaDataSource() datasource.DataSource { return &SchemaDataSource{} }

func (d *SchemaDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schema"
}

func (d *SchemaDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a Brockley library schema.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Required: true},
			"name":        schema.StringAttribute{Computed: true},
			"namespace":   schema.StringAttribute{Computed: true},
			"description": schema.StringAttribute{Computed: true},
			"json_schema": schema.StringAttribute{Computed: true},
		},
	}
}

func (d *SchemaDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

type schemaDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Namespace   types.String `tfsdk:"namespace"`
	Description types.String `tfsdk:"description"`
	JSONSchema  types.String `tfsdk:"json_schema"`
}

func (d *SchemaDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg schemaDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := d.client.GetSchema(ctx, cfg.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read schema", err.Error())
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
	if v, ok := m["json_schema"]; ok {
		b, _ := json.Marshal(v)
		cfg.JSONSchema = types.StringValue(string(b))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// PromptTemplateDataSource reads a brockley prompt template.
type PromptTemplateDataSource struct {
	client *client.Client
}

var _ datasource.DataSource = (*PromptTemplateDataSource)(nil)

func NewPromptTemplateDataSource() datasource.DataSource { return &PromptTemplateDataSource{} }

func (d *PromptTemplateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_prompt_template"
}

func (d *PromptTemplateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a Brockley prompt template.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Required: true},
			"name":            schema.StringAttribute{Computed: true},
			"namespace":       schema.StringAttribute{Computed: true},
			"description":     schema.StringAttribute{Computed: true},
			"system_prompt":   schema.StringAttribute{Computed: true},
			"user_prompt":     schema.StringAttribute{Computed: true},
			"variables":       schema.StringAttribute{Computed: true},
			"response_format": schema.StringAttribute{Computed: true},
		},
	}
}

func (d *PromptTemplateDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

type promptTemplateDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Namespace      types.String `tfsdk:"namespace"`
	Description    types.String `tfsdk:"description"`
	SystemPrompt   types.String `tfsdk:"system_prompt"`
	UserPrompt     types.String `tfsdk:"user_prompt"`
	Variables      types.String `tfsdk:"variables"`
	ResponseFormat types.String `tfsdk:"response_format"`
}

func (d *PromptTemplateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg promptTemplateDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := d.client.GetPromptTemplate(ctx, cfg.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read prompt template", err.Error())
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
	if v, ok := m["system_prompt"].(string); ok {
		cfg.SystemPrompt = types.StringValue(v)
	}
	if v, ok := m["user_prompt"].(string); ok {
		cfg.UserPrompt = types.StringValue(v)
	}
	if v, ok := m["variables"]; ok {
		b, _ := json.Marshal(v)
		cfg.Variables = types.StringValue(string(b))
	}
	if v, ok := m["response_format"].(string); ok {
		cfg.ResponseFormat = types.StringValue(v)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// ProviderConfigDataSource reads a brockley provider config.
type ProviderConfigDataSource struct {
	client *client.Client
}

var _ datasource.DataSource = (*ProviderConfigDataSource)(nil)

func NewProviderConfigDataSource() datasource.DataSource { return &ProviderConfigDataSource{} }

func (d *ProviderConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_provider_config"
}

func (d *ProviderConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a Brockley provider config.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Required: true},
			"name":          schema.StringAttribute{Computed: true},
			"namespace":     schema.StringAttribute{Computed: true},
			"provider_type": schema.StringAttribute{Computed: true},
			"base_url":      schema.StringAttribute{Computed: true},
			"api_key_ref":   schema.StringAttribute{Computed: true},
			"default_model": schema.StringAttribute{Computed: true},
		},
	}
}

func (d *ProviderConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

type providerConfigDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Namespace    types.String `tfsdk:"namespace"`
	Provider     types.String `tfsdk:"provider_type"`
	BaseURL      types.String `tfsdk:"base_url"`
	APIKeyRef    types.String `tfsdk:"api_key_ref"`
	DefaultModel types.String `tfsdk:"default_model"`
}

func (d *ProviderConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg providerConfigDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := d.client.GetProviderConfig(ctx, cfg.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read provider config", err.Error())
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
	if v, ok := m["provider"].(string); ok {
		cfg.Provider = types.StringValue(v)
	}
	if v, ok := m["base_url"].(string); ok {
		cfg.BaseURL = types.StringValue(v)
	}
	if v, ok := m["api_key_ref"].(string); ok {
		cfg.APIKeyRef = types.StringValue(v)
	}
	if v, ok := m["default_model"].(string); ok {
		cfg.DefaultModel = types.StringValue(v)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
