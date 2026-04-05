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

var _ resource.Resource = (*SchemaResource)(nil)
var _ resource.ResourceWithImportState = (*SchemaResource)(nil)

// SchemaResource manages a brockley_schema.
type SchemaResource struct {
	client *client.Client
}

// SchemaResourceModel is the Terraform schema model for a library schema.
type SchemaResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Namespace   types.String `tfsdk:"namespace"`
	Description types.String `tfsdk:"description"`
	JSONSchema  types.String `tfsdk:"json_schema"`
}

func NewSchemaResource() resource.Resource {
	return &SchemaResource{}
}

func (r *SchemaResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schema"
}

func (r *SchemaResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Brockley library schema.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Schema name.",
			},
			"namespace": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Namespace.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Schema description.",
			},
			"json_schema": schema.StringAttribute{
				Required:    true,
				Description: "JSON Schema definition (JSON-encoded string).",
			},
		},
	}
}

func (r *SchemaResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SchemaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SchemaResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := buildSchemaBody(plan)
	result, err := r.client.CreateSchema(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create schema", err.Error())
		return
	}

	populateSchemaModel(&plan, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SchemaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SchemaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.GetSchema(ctx, state.ID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read schema", err.Error())
		return
	}

	populateSchemaModel(&state, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SchemaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SchemaResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state SchemaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := buildSchemaBody(plan)
	result, err := r.client.UpdateSchema(ctx, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update schema", err.Error())
		return
	}

	populateSchemaModel(&plan, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SchemaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SchemaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteSchema(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete schema", err.Error())
	}
}

func (r *SchemaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	result, err := r.client.GetSchema(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import schema", err.Error())
		return
	}
	var model SchemaResourceModel
	populateSchemaModel(&model, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func buildSchemaBody(plan SchemaResourceModel) map[string]any {
	body := map[string]any{"name": plan.Name.ValueString()}
	if !plan.Namespace.IsNull() && !plan.Namespace.IsUnknown() {
		body["namespace"] = plan.Namespace.ValueString()
	}
	if !plan.Description.IsNull() {
		body["description"] = plan.Description.ValueString()
	}
	if !plan.JSONSchema.IsNull() {
		var s any
		json.Unmarshal([]byte(plan.JSONSchema.ValueString()), &s)
		body["json_schema"] = s
	}
	return body
}

func populateSchemaModel(model *SchemaResourceModel, data json.RawMessage) {
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
	if v, ok := m["json_schema"]; ok {
		b, _ := json.Marshal(v)
		model.JSONSchema = types.StringValue(string(b))
	}
}
