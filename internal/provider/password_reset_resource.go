package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/everadaptive/terraform-provider-purelymail/internal/purelymail"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithImportState = &passwordResetResource{}

func NewPasswordResetResource() resource.Resource {
	return &passwordResetResource{}
}

type passwordResetResource struct {
	client *purelymail.Client
}

type passwordResetResourceModel struct {
	ID            types.String `tfsdk:"id"`
	UserName      types.String `tfsdk:"user_name"`
	Type          types.String `tfsdk:"type"`
	Target        types.String `tfsdk:"target"`
	Description   types.String `tfsdk:"description"`
	AllowMfaReset types.Bool   `tfsdk:"allow_mfa_reset"`
}

func (r *passwordResetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_password_reset"
}

func (r *passwordResetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a password reset method (email or phone) for a user.",
		Attributes: map[string]schema.Attribute{
			"user_name": schema.StringAttribute{
				MarkdownDescription: "Full username of the mailbox owner, e.g. `user@example.com`.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Type of reset method, either `email` or `phone`. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"target": schema.StringAttribute{
				MarkdownDescription: "Email address or phone number the reset is sent to.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Human-readable description of the method.",
				Optional:            true,
				Computed:            true,
			},
			"allow_mfa_reset": schema.BoolAttribute{
				MarkdownDescription: "Whether this method may also reset the user's MFA. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Import id in the form `<user_name>:<target>`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *passwordResetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	clients, ok := req.ProviderData.(*ProviderClients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected resource configure type",
			fmt.Sprintf("Expected *ProviderClients, got: %T. Please report this issue.", req.ProviderData),
		)
		return
	}
	r.client = clients.API
}

func (r *passwordResetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan passwordResetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpsertPasswordReset(ctx, purelymail.UpsertPasswordResetRequest{
		UserName:      plan.UserName.ValueString(),
		Type:          plan.Type.ValueString(),
		Target:        plan.Target.ValueString(),
		Description:   plan.Description.ValueString(),
		AllowMfaReset: boolPtrAddr(plan.AllowMfaReset),
	}); err != nil {
		resp.Diagnostics.AddError("Unable to create password reset method", err.Error())
		return
	}

	if !r.read(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *passwordResetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state passwordResetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	found := r.read(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// read refreshes the model from the API by matching the target in the user's
// reset method list. It returns false if the method no longer exists.
func (r *passwordResetResource) read(ctx context.Context, m *passwordResetResourceModel, diags *diag.Diagnostics) bool {
	methods, err := r.client.ListPasswordReset(ctx, purelymail.GetUserRequest{
		UserName: m.UserName.ValueString(),
	})
	if err != nil {
		diags.AddError("Unable to list password reset methods", err.Error())
		return false
	}

	for _, method := range methods {
		if method.Target != m.Target.ValueString() {
			continue
		}
		m.ID = types.StringValue(m.UserName.ValueString() + ":" + m.Target.ValueString())
		m.Type = types.StringValue(method.Type)
		m.Description = types.StringValue(method.Description)
		m.AllowMfaReset = types.BoolValue(method.AllowMfaReset)
		return true
	}
	return false
}

func (r *passwordResetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state passwordResetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Upsert by the existing target so the method is updated in place.
	if err := r.client.UpsertPasswordReset(ctx, purelymail.UpsertPasswordResetRequest{
		UserName:       plan.UserName.ValueString(),
		ExistingTarget: state.Target.ValueString(),
		Type:           plan.Type.ValueString(),
		Target:         plan.Target.ValueString(),
		Description:    plan.Description.ValueString(),
		AllowMfaReset:  boolPtrAddr(plan.AllowMfaReset),
	}); err != nil {
		resp.Diagnostics.AddError("Unable to update password reset method", err.Error())
		return
	}

	if !r.read(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *passwordResetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state passwordResetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeletePasswordReset(ctx, purelymail.DeletePasswordResetRequest{
		UserName: state.UserName.ValueString(),
		Target:   state.Target.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Unable to delete password reset method", err.Error())
	}
}

func (r *passwordResetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// The import id is "<user_name>:<target>" (targets may contain dots but
	// not colons).
	user, target, found := strings.Cut(req.ID, ":")
	if !found {
		resp.Diagnostics.AddError(
			"Invalid import id",
			fmt.Sprintf("Expected import id of the form <user_name>:<target>, got %q.", req.ID),
		)
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_name"), user)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("target"), target)...)
}
