package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brockleyai/brockleyai/cli/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*GraphResource)(nil)
var _ resource.ResourceWithImportState = (*GraphResource)(nil)

// GraphResource manages a brockley_graph.
type GraphResource struct {
	client *client.Client
}

// GraphResourceModel is the Terraform schema model for a graph.
type GraphResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Namespace   types.String `tfsdk:"namespace"`
	Description types.String `tfsdk:"description"`
	Status      types.String `tfsdk:"status"`
	Version     types.Int64  `tfsdk:"version"`
	Nodes       types.String `tfsdk:"nodes"`
	Edges       types.String `tfsdk:"edges"`
	State       types.String `tfsdk:"state"`
}

func NewGraphResource() resource.Resource {
	return &GraphResource{}
}

func (r *GraphResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_graph"
}

func (r *GraphResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Brockley agent graph.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Graph ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Graph name.",
			},
			"namespace": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Namespace (default: 'default').",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Graph description.",
			},
			"status": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Graph status: draft, active, archived.",
			},
			"version": schema.Int64Attribute{
				Computed:    true,
				Description: "Graph version (auto-incremented on update).",
			},
			"nodes": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "JSON-encoded array of node definitions. Marked sensitive because nodes may contain inline API keys.",
			},
			"edges": schema.StringAttribute{
				Optional:    true,
				Description: "JSON-encoded array of edge definitions.",
			},
			"state": schema.StringAttribute{
				Optional:    true,
				Description: "JSON-encoded graph state definition.",
			},
		},
	}
}

func (r *GraphResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", "Expected *client.Client")
		return
	}
	r.client = c
}

func (r *GraphResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan GraphResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := buildGraphBody(plan)

	result, err := r.client.CreateGraph(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create graph", err.Error())
		return
	}

	// Preserve the user-provided JSON strings (nodes, edges, state) so
	// Terraform doesn't flag re-serialized key-order differences as drift.
	savedNodes := plan.Nodes
	savedEdges := plan.Edges
	savedState := plan.State
	populateGraphModel(&plan, result)
	if !savedNodes.IsNull() && !savedNodes.IsUnknown() {
		plan.Nodes = savedNodes
	}
	if !savedEdges.IsNull() && !savedEdges.IsUnknown() {
		plan.Edges = savedEdges
	}
	if !savedState.IsNull() && !savedState.IsUnknown() {
		plan.State = savedState
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *GraphResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state GraphResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.GetGraph(ctx, state.ID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read graph", err.Error())
		return
	}

	// Preserve nodes from Terraform state because the server returns masked API keys.
	// Only update non-sensitive computed fields from the server response.
	savedNodes := state.Nodes
	populateGraphModel(&state, result)
	if !savedNodes.IsNull() && !savedNodes.IsUnknown() {
		state.Nodes = savedNodes
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *GraphResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan GraphResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state GraphResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := buildGraphBody(plan)

	result, err := r.client.UpdateGraph(ctx, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update graph", err.Error())
		return
	}

	savedNodes := plan.Nodes
	savedEdges := plan.Edges
	savedState := plan.State
	populateGraphModel(&plan, result)
	if !savedNodes.IsNull() && !savedNodes.IsUnknown() {
		plan.Nodes = savedNodes
	}
	if !savedEdges.IsNull() && !savedEdges.IsUnknown() {
		plan.Edges = savedEdges
	}
	if !savedState.IsNull() && !savedState.IsUnknown() {
		plan.State = savedState
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *GraphResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state GraphResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteGraph(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete graph", err.Error())
	}
}

func (r *GraphResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	result, err := r.client.GetGraph(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import graph", err.Error())
		return
	}

	var model GraphResourceModel
	populateGraphModel(&model, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func buildGraphBody(plan GraphResourceModel) map[string]any {
	body := map[string]any{
		"name": plan.Name.ValueString(),
	}
	if !plan.Namespace.IsNull() && !plan.Namespace.IsUnknown() {
		body["namespace"] = plan.Namespace.ValueString()
	}
	if !plan.Description.IsNull() {
		body["description"] = plan.Description.ValueString()
	}
	if !plan.Status.IsNull() && !plan.Status.IsUnknown() {
		body["status"] = plan.Status.ValueString()
	}
	if !plan.Nodes.IsNull() {
		var nodes any
		json.Unmarshal([]byte(plan.Nodes.ValueString()), &nodes)
		body["nodes"] = nodes
	}
	if !plan.Edges.IsNull() {
		var edges any
		json.Unmarshal([]byte(plan.Edges.ValueString()), &edges)
		body["edges"] = edges
	}
	if !plan.State.IsNull() {
		var state any
		json.Unmarshal([]byte(plan.State.ValueString()), &state)
		body["state"] = state
	}
	return body
}

func populateGraphModel(model *GraphResourceModel, data json.RawMessage) {
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
	if v, ok := m["status"].(string); ok {
		model.Status = types.StringValue(v)
	}
	if v, ok := m["version"].(float64); ok {
		model.Version = types.Int64Value(int64(v))
	}
	if v, ok := m["nodes"]; ok {
		b, _ := json.Marshal(v)
		model.Nodes = types.StringValue(string(b))
	}
	if v, ok := m["edges"]; ok {
		b, _ := json.Marshal(v)
		model.Edges = types.StringValue(string(b))
	}
	if v, ok := m["state"]; ok && v != nil {
		b, _ := json.Marshal(v)
		model.State = types.StringValue(string(b))
	}
}

// For extracting strings from JSON in tests
func jsonStr(data json.RawMessage, key string) string {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}
