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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithImportState = &userResource{}

func NewUserResource() resource.Resource {
	return &userResource{}
}

type userResource struct {
	client *purelymail.Client
}

type userResourceModel struct {
	ID                             types.String `tfsdk:"id"`
	UserName                       types.String `tfsdk:"user_name"`
	DomainName                     types.String `tfsdk:"domain_name"`
	Password                       types.String `tfsdk:"password"`
	EnablePasswordReset            types.Bool   `tfsdk:"enable_password_reset"`
	EnableSearchIndexing           types.Bool   `tfsdk:"enable_search_indexing"`
	RequireTwoFactorAuthentication types.Bool   `tfsdk:"require_two_factor_authentication"`
	EnableSpamFiltering            types.Bool   `tfsdk:"enable_spam_filtering"`
}

func (r *userResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *userResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a mailbox user in your PurelyMail account.",
		Attributes: map[string]schema.Attribute{
			"user_name": schema.StringAttribute{
				MarkdownDescription: "Local part of the username, e.g. `user` in `user@example.com`.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"domain_name": schema.StringAttribute{
				MarkdownDescription: "Domain of the mailbox, e.g. `example.com`.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Mailbox password. Required on create; on import it stays null " +
					"and is only sent when changed (the API never returns passwords).",
				Optional:  true,
				Sensitive: true,
			},
			"enable_password_reset": schema.BoolAttribute{
				// The API never returns this flag, so it cannot be computed;
				// it is only sent on create/update.
				MarkdownDescription: "Whether password reset methods may be used.",
				Optional:            true,
			},
			"enable_search_indexing": schema.BoolAttribute{
				MarkdownDescription: "Whether the mailbox is indexed for search.",
				Optional:            true,
				Computed:            true,
			},
			"require_two_factor_authentication": schema.BoolAttribute{
				MarkdownDescription: "Whether two-factor authentication is required.",
				Optional:            true,
				Computed:            true,
			},
			"enable_spam_filtering": schema.BoolAttribute{
				MarkdownDescription: "Whether spam filtering is enabled (read-only).",
				Computed:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Full username (`user_name@domain_name`).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Password.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("password"),
			"Password required",
			"A password must be set when creating a purelymail_user.",
		)
		return
	}

	create := purelymail.CreateUserRequest{
		UserName:             plan.UserName.ValueString(),
		DomainName:           plan.DomainName.ValueString(),
		Password:             plan.Password.ValueString(),
		EnablePasswordReset:  boolPtrAddr(plan.EnablePasswordReset),
		EnableSearchIndexing: boolPtrAddr(plan.EnableSearchIndexing),
	}
	if err := r.client.CreateUser(ctx, create); err != nil {
		// If the user already exists (e.g. a previous create succeeded but
		// state was not saved), adopt it instead of failing with an orphan.
		existing, findErr := r.client.GetUser(ctx, purelymail.GetUserRequest{
			UserName: fullUserName(&plan),
		})
		if findErr != nil || existing == nil {
			resp.Diagnostics.AddError("Unable to create user", err.Error())
			return
		}
		resp.Diagnostics.AddWarning(
			"User already existed",
			fmt.Sprintf("User %q already existed and has been adopted into state.", fullUserName(&plan)),
		)
	}

	if !r.read(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Import passes the full username as `id`; split it into components.
	if state.UserName.IsNull() || state.DomainName.IsNull() {
		local, domain, err := purelymail.SplitUserName(state.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to import user", err.Error())
			return
		}
		state.UserName = types.StringValue(local)
		state.DomainName = types.StringValue(domain)
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

// read refreshes user attributes from the API. It returns false if the user
// no longer exists; API failures are added to diagnostics.
func (r *userResource) read(ctx context.Context, m *userResourceModel, diags *diag.Diagnostics) bool {
	full := fullUserName(m)

	user, err := r.client.GetUser(ctx, purelymail.GetUserRequest{UserName: full})
	if err != nil {
		diags.AddError("Unable to read user", err.Error())
		return false
	}

	m.ID = types.StringValue(full)
	m.EnableSearchIndexing = boolPtr(user.EnableSearchIndexing)
	m.RequireTwoFactorAuthentication = boolPtr(user.RequireTwoFactorAuthentication)
	m.EnableSpamFiltering = boolPtr(user.EnableSpamFiltering)
	return true
}

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state userResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	update := purelymail.ModifyUserRequest{
		UserName: fullUserName(&state),
	}
	if !plan.EnablePasswordReset.Equal(state.EnablePasswordReset) {
		update.EnablePasswordReset = boolPtrAddr(plan.EnablePasswordReset)
	}
	if !plan.EnableSearchIndexing.Equal(state.EnableSearchIndexing) {
		update.EnableSearchIndexing = boolPtrAddr(plan.EnableSearchIndexing)
	}
	if !plan.RequireTwoFactorAuthentication.Equal(state.RequireTwoFactorAuthentication) {
		update.RequireTwoFactorAuthentication = boolPtrAddr(plan.RequireTwoFactorAuthentication)
	}
	// The API never returns passwords, so password is only meaningful when it
	// transitions from unset to set (or changes to a new value).
	if !plan.Password.Equal(state.Password) && !plan.Password.IsNull() {
		update.NewPassword = plan.Password.ValueString()
	} else if !plan.Password.Equal(state.Password) && plan.Password.IsNull() {
		// Cannot clear a password; ignore and keep the existing one.
		plan.Password = types.StringNull()
	}

	if err := r.client.ModifyUser(ctx, update); err != nil {
		resp.Diagnostics.AddError("Unable to modify user", err.Error())
		return
	}

	if !r.read(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteUser(ctx, purelymail.DeleteUserRequest{
		UserName: fullUserName(&state),
	}); err != nil {
		// Usernames are case-insensitive server-side but may differ in case
		// from state; a missing user on delete is treated as success.
		if !strings.Contains(err.Error(), "NoSuchUser") {
			resp.Diagnostics.AddError("Unable to delete user", err.Error())
		}
	}
}

func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func fullUserName(m *userResourceModel) string {
	return m.UserName.ValueString() + "@" + m.DomainName.ValueString()
}

func boolPtrAddr(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}
