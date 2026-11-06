package provider

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"strconv"
	"time"

	"github.com/everadaptive/terraform-provider-purelymail/internal/purelymail"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource = &appPasswordResource{}
)

func NewAppPasswordResource() resource.Resource {
	return &appPasswordResource{}
}

type appPasswordResource struct {
	client *purelymail.Client
}

type appPasswordResourceModel struct {
	ID           types.String `tfsdk:"id"`
	UserHandle   types.String `tfsdk:"user_handle"`
	Name         types.String `tfsdk:"name"`
	ResolvedName types.String `tfsdk:"resolved_name"`
	AppPassword  types.String `tfsdk:"app_password"`
}

func (r *appPasswordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_password"
}

func (r *appPasswordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	// The API provides no way to list app passwords, so the resource cannot
	// refresh state: Read is a no-op and the password lives only in state.
	// Any change re-creates the app password (a new password is generated).
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an app password for a user. The API cannot list app " +
			"passwords, so the password is only known at creation time and state " +
			"refresh is not possible. Changing `name` or `user_handle` creates a " +
			"new app password.",
		Attributes: map[string]schema.Attribute{
			"user_handle": schema.StringAttribute{
				MarkdownDescription: "Full username of the mailbox owner, e.g. `user@example.com`.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				// Names are not unique server-side and app passwords cannot
				// be listed, so a random suffix is appended at create time
				// to make orphan cleanup in the web UI unambiguous; the final
				// name is exposed as resolved_name.
				MarkdownDescription: "User-friendly description of the app password. A random suffix " +
					"is appended at create time so each password is identifiable in the web UI.",
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"resolved_name": schema.StringAttribute{
				MarkdownDescription: "The final name registered with the server (config `name` plus a random suffix).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_password": schema.StringAttribute{
				MarkdownDescription: "The generated app password (sensitive). Only available in state.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Same as `app_password` (which is the deletion key).",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *appPasswordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *appPasswordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan appPasswordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Names are not unique server-side and there is no list endpoint, so a
	// random suffix makes each provider-created password identifiable for
	// orphan cleanup in the web UI.
	name := plan.Name.ValueString()
	if name == "" {
		name = "terraform"
	}
	name = name + "-" + randSuffix(6)

	out, err := r.client.CreateAppPassword(ctx, purelymail.CreateAppPasswordRequest{
		UserHandle: plan.UserHandle.ValueString(),
		Name:       name,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create app password", err.Error())
		return
	}

	plan.ResolvedName = types.StringValue(name)
	plan.AppPassword = types.StringValue(out.AppPassword)
	plan.ID = types.StringValue(out.AppPassword)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// randSuffix returns a random lowercase alphanumeric suffix.
func randSuffix(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	if _, err := crand.Read(b); err != nil {
		// Unreachable in practice; fall back to time-based uniqueness.
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

func (r *appPasswordResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
	// The API cannot list or fetch app passwords, so there is nothing to
	// refresh; state is kept as-is.
}

func (r *appPasswordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// user_handle and name force replacement, so Update is never reached.
	resp.Diagnostics.AddError(
		"Unexpected update",
		"purelymail_app_password does not support in-place updates; all attributes force replacement. Please report this issue.",
	)
}

func (r *appPasswordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appPasswordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteAppPassword(ctx, purelymail.DeleteAppPasswordRequest{
		UserName:    state.UserHandle.ValueString(),
		AppPassword: state.AppPassword.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Unable to delete app password", err.Error())
	}
}
