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

var _ resource.Resource = (*ProviderConfigResource)(nil)
var _ resource.ResourceWithImportState = (*ProviderConfigResource)(nil)

// ProviderConfigResource manages a brockley_provider_config.
type ProviderConfigResource struct {
	client *client.Client
}

// ProviderConfigResourceModel is the Terraform model.
type ProviderConfigResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Namespace    types.String `tfsdk:"namespace"`
	Provider     types.String `tfsdk:"provider_type"`
	BaseURL      types.String `tfsdk:"base_url"`
	APIKeyRef    types.String `tfsdk:"api_key_ref"`
	DefaultModel types.String `tfsdk:"default_model"`
}

func NewProviderConfigResource() resource.Resource {
	return &ProviderConfigResource{}
}

func (r *ProviderConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_provider_config"
}

func (r *ProviderConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Brockley LLM provider configuration.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"namespace": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"provider_type": schema.StringAttribute{
				Required:    true,
				Description: "LLM provider: openai, anthropic, google, openrouter, bedrock.",
			},
			"base_url": schema.StringAttribute{
				Optional:    true,
				Description: "Custom base URL for the provider API.",
			},
			"api_key_ref": schema.StringAttribute{
				Required:    true,
				Description: "Secret store reference for the API key.",
			},
			"default_model": schema.StringAttribute{
				Optional:    true,
				Description: "Default model to use.",
			},
		},
	}
}

func (r *ProviderConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ProviderConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ProviderConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := buildProviderConfigBody(plan)
	result, err := r.client.CreateProviderConfig(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create provider config", err.Error())
		return
	}

	populateProviderConfigModel(&plan, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ProviderConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ProviderConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.GetProviderConfig(ctx, state.ID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read provider config", err.Error())
		return
	}

	populateProviderConfigModel(&state, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ProviderConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ProviderConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state ProviderConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := buildProviderConfigBody(plan)
	result, err := r.client.UpdateProviderConfig(ctx, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update provider config", err.Error())
		return
	}

	populateProviderConfigModel(&plan, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ProviderConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ProviderConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteProviderConfig(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete provider config", err.Error())
	}
}

func (r *ProviderConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	result, err := r.client.GetProviderConfig(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import provider config", err.Error())
		return
	}
	var model ProviderConfigResourceModel
	populateProviderConfigModel(&model, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func buildProviderConfigBody(plan ProviderConfigResourceModel) map[string]any {
	body := map[string]any{
		"name":        plan.Name.ValueString(),
		"provider":    plan.Provider.ValueString(),
		"api_key_ref": plan.APIKeyRef.ValueString(),
	}
	if !plan.Namespace.IsNull() && !plan.Namespace.IsUnknown() {
		body["namespace"] = plan.Namespace.ValueString()
	}
	if !plan.BaseURL.IsNull() {
		body["base_url"] = plan.BaseURL.ValueString()
	}
	if !plan.DefaultModel.IsNull() {
		body["default_model"] = plan.DefaultModel.ValueString()
	}
	return body
}

func populateProviderConfigModel(model *ProviderConfigResourceModel, data json.RawMessage) {
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
	if v, ok := m["provider"].(string); ok {
		model.Provider = types.StringValue(v)
	}
	if v, ok := m["base_url"].(string); ok {
		model.BaseURL = types.StringValue(v)
	}
	if v, ok := m["api_key_ref"].(string); ok {
		model.APIKeyRef = types.StringValue(v)
	}
	if v, ok := m["default_model"].(string); ok {
		model.DefaultModel = types.StringValue(v)
	}
}
