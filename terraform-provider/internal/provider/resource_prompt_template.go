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

var _ resource.Resource = (*PromptTemplateResource)(nil)
var _ resource.ResourceWithImportState = (*PromptTemplateResource)(nil)

// PromptTemplateResource manages a brockley_prompt_template.
type PromptTemplateResource struct {
	client *client.Client
}

// PromptTemplateResourceModel is the Terraform model.
type PromptTemplateResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Namespace      types.String `tfsdk:"namespace"`
	Description    types.String `tfsdk:"description"`
	SystemPrompt   types.String `tfsdk:"system_prompt"`
	UserPrompt     types.String `tfsdk:"user_prompt"`
	Variables      types.String `tfsdk:"variables"`
	ResponseFormat types.String `tfsdk:"response_format"`
}

func NewPromptTemplateResource() resource.Resource {
	return &PromptTemplateResource{}
}

func (r *PromptTemplateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_prompt_template"
}

func (r *PromptTemplateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Brockley prompt template.",
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
			"description": schema.StringAttribute{
				Optional: true,
			},
			"system_prompt": schema.StringAttribute{
				Optional:    true,
				Description: "System prompt text.",
			},
			"user_prompt": schema.StringAttribute{
				Required:    true,
				Description: "User prompt template.",
			},
			"variables": schema.StringAttribute{
				Optional:    true,
				Description: "JSON-encoded array of variable definitions.",
			},
			"response_format": schema.StringAttribute{
				Optional:    true,
				Description: "Response format: text or json.",
			},
		},
	}
}

func (r *PromptTemplateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *PromptTemplateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan PromptTemplateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := buildPromptTemplateBody(plan)
	result, err := r.client.CreatePromptTemplate(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create prompt template", err.Error())
		return
	}

	populatePromptTemplateModel(&plan, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PromptTemplateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state PromptTemplateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.GetPromptTemplate(ctx, state.ID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read prompt template", err.Error())
		return
	}

	populatePromptTemplateModel(&state, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *PromptTemplateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan PromptTemplateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state PromptTemplateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := buildPromptTemplateBody(plan)
	result, err := r.client.UpdatePromptTemplate(ctx, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update prompt template", err.Error())
		return
	}

	populatePromptTemplateModel(&plan, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PromptTemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state PromptTemplateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeletePromptTemplate(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete prompt template", err.Error())
	}
}

func (r *PromptTemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	result, err := r.client.GetPromptTemplate(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import prompt template", err.Error())
		return
	}
	var model PromptTemplateResourceModel
	populatePromptTemplateModel(&model, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func buildPromptTemplateBody(plan PromptTemplateResourceModel) map[string]any {
	body := map[string]any{
		"name":        plan.Name.ValueString(),
		"user_prompt": plan.UserPrompt.ValueString(),
	}
	if !plan.Namespace.IsNull() && !plan.Namespace.IsUnknown() {
		body["namespace"] = plan.Namespace.ValueString()
	}
	if !plan.Description.IsNull() {
		body["description"] = plan.Description.ValueString()
	}
	if !plan.SystemPrompt.IsNull() {
		body["system_prompt"] = plan.SystemPrompt.ValueString()
	}
	if !plan.Variables.IsNull() {
		var v any
		json.Unmarshal([]byte(plan.Variables.ValueString()), &v)
		body["variables"] = v
	}
	if !plan.ResponseFormat.IsNull() {
		body["response_format"] = plan.ResponseFormat.ValueString()
	}
	return body
}

func populatePromptTemplateModel(model *PromptTemplateResourceModel, data json.RawMessage) {
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
	if v, ok := m["system_prompt"].(string); ok {
		model.SystemPrompt = types.StringValue(v)
	}
	if v, ok := m["user_prompt"].(string); ok {
		model.UserPrompt = types.StringValue(v)
	}
	if v, ok := m["variables"]; ok {
		b, _ := json.Marshal(v)
		model.Variables = types.StringValue(string(b))
	}
	if v, ok := m["response_format"].(string); ok {
		model.ResponseFormat = types.StringValue(v)
	}
}
