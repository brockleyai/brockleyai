package provider

import (
	"context"
	"encoding/json"

	"github.com/brockleyai/brockleyai/cli/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*APIToolResource)(nil)
var _ resource.ResourceWithImportState = (*APIToolResource)(nil)

// APIToolResource manages a brockley_api_tool.
type APIToolResource struct {
	client *client.Client
}

// APIToolResourceModel is the Terraform model for an API tool definition.
type APIToolResourceModel struct {
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

func NewAPIToolResource() resource.Resource {
	return &APIToolResource{}
}

func (r *APIToolResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_tool"
}

func (r *APIToolResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Brockley API tool definition.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "API tool name.",
			},
			"namespace": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Namespace (defaults to \"default\").",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "API tool description.",
			},
			"base_url": schema.StringAttribute{
				Required:    true,
				Description: "Base URL for the API.",
			},
			"default_timeout_ms": schema.Int64Attribute{
				Optional:    true,
				Description: "Default timeout in milliseconds (0 = 30s default).",
			},
			"default_headers": schema.StringAttribute{
				Optional:    true,
				Description: "JSON-encoded array of default header configs.",
			},
			"retry": schema.StringAttribute{
				Optional:    true,
				Description: "JSON-encoded retry configuration.",
			},
			"endpoints": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "JSON-encoded array of API endpoint definitions. Marked sensitive because endpoint headers may contain secret values.",
			},
			"metadata": schema.StringAttribute{
				Optional:    true,
				Description: "JSON-encoded metadata object.",
			},
		},
	}
}

func (r *APIToolResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected type", "Expected *client.Client")
		return
	}
	r.client = c
}

func (r *APIToolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan APIToolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := buildAPIToolBody(plan)
	result, err := r.client.CreateAPITool(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create API tool", err.Error())
		return
	}

	populateAPIToolModel(&plan, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *APIToolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state APIToolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.GetAPITool(ctx, state.ID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read API tool", err.Error())
		return
	}

	populateAPIToolModel(&state, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *APIToolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan APIToolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state APIToolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := buildAPIToolBody(plan)
	result, err := r.client.UpdateAPITool(ctx, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update API tool", err.Error())
		return
	}

	populateAPIToolModel(&plan, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *APIToolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state APIToolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteAPITool(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete API tool", err.Error())
	}
}

func (r *APIToolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	result, err := r.client.GetAPITool(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import API tool", err.Error())
		return
	}
	var model APIToolResourceModel
	populateAPIToolModel(&model, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func buildAPIToolBody(plan APIToolResourceModel) map[string]any {
	body := map[string]any{
		"name":     plan.Name.ValueString(),
		"base_url": plan.BaseURL.ValueString(),
	}
	if !plan.Namespace.IsNull() && !plan.Namespace.IsUnknown() {
		body["namespace"] = plan.Namespace.ValueString()
	}
	if !plan.Description.IsNull() {
		body["description"] = plan.Description.ValueString()
	}
	if !plan.DefaultTimeoutMs.IsNull() {
		body["default_timeout_ms"] = plan.DefaultTimeoutMs.ValueInt64()
	}
	if !plan.DefaultHeaders.IsNull() {
		var v any
		json.Unmarshal([]byte(plan.DefaultHeaders.ValueString()), &v)
		body["default_headers"] = v
	}
	if !plan.Retry.IsNull() {
		var v any
		json.Unmarshal([]byte(plan.Retry.ValueString()), &v)
		body["retry"] = v
	}
	if !plan.Endpoints.IsNull() {
		var v any
		json.Unmarshal([]byte(plan.Endpoints.ValueString()), &v)
		body["endpoints"] = v
	}
	if !plan.Metadata.IsNull() {
		var v any
		json.Unmarshal([]byte(plan.Metadata.ValueString()), &v)
		body["metadata"] = v
	}
	return body
}

func populateAPIToolModel(model *APIToolResourceModel, data json.RawMessage) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}
	if v, ok := m["id"].(string); ok {
		model.ID = types.StringValue(v)
	}
	if v, ok := m["name"].(string); ok {
		model.Name = types.StringValue(v)
	}
	if v, ok := m["namespace"].(string); ok {
		model.Namespace = types.StringValue(v)
	}
	if v, ok := m["description"].(string); ok {
		model.Description = types.StringValue(v)
	}
	if v, ok := m["base_url"].(string); ok {
		model.BaseURL = types.StringValue(v)
	}
	if v, ok := m["default_timeout_ms"].(float64); ok && v != 0 {
		model.DefaultTimeoutMs = types.Int64Value(int64(v))
	}
	if v, ok := m["default_headers"]; ok {
		b, _ := json.Marshal(v)
		model.DefaultHeaders = types.StringValue(string(b))
	}
	if v, ok := m["retry"]; ok && v != nil {
		b, _ := json.Marshal(v)
		model.Retry = types.StringValue(string(b))
	}
	if v, ok := m["endpoints"]; ok {
		b, _ := json.Marshal(v)
		model.Endpoints = types.StringValue(string(b))
	}
	if v, ok := m["metadata"]; ok && v != nil {
		b, _ := json.Marshal(v)
		model.Metadata = types.StringValue(string(b))
	}
}
