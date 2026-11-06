package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/everadaptive/terraform-provider-purelymail/internal/sieve"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithImportState = &sieveScriptResource{}

func NewSieveScriptResource() resource.Resource {
	return &sieveScriptResource{}
}

type sieveScriptResource struct {
	sieveHost string
}

type sieveScriptResourceModel struct {
	ID         types.String `tfsdk:"id"`
	UserHandle types.String `tfsdk:"user_handle"`
	Password   types.String `tfsdk:"password"`
	Name       types.String `tfsdk:"name"`
	Content    types.String `tfsdk:"content"`
	Active     types.Bool   `tfsdk:"active"`
}

func (r *sieveScriptResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sieve_script"
}

func (r *sieveScriptResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	// Sieve scripts are managed over the ManageSieve protocol (RFC 5804),
	// not the PurelyMail API, so authentication is per-resource with the
	// mailbox password (an app password works well).
	//
	// The server does not support GETSCRIPT, so script content cannot be
	// read back and drift on `content` is undetectable; existence and the
	// active flag are refreshed via LISTSCRIPTS.
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Sieve filter script for a user via the Manage Sieve " +
			"protocol. Requires the mailbox password (an app password is ideal); " +
			"the API token is not used for Sieve. Script content cannot be read " +
			"back from the server, so drift on `content` is undetectable.",
		Attributes: map[string]schema.Attribute{
			"user_handle": schema.StringAttribute{
				MarkdownDescription: "Full username of the mailbox owner, e.g. `user@example.com`.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Mailbox password used to authenticate to ManageSieve. An app password works and is recommended.",
				Optional:            true,
				Sensitive:           true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Script name, e.g. `main`. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"content": schema.StringAttribute{
				MarkdownDescription: "Sieve script source. Cannot be read back from the server.",
				Required:            true,
			},
			"active": schema.BoolAttribute{
				MarkdownDescription: "Whether the script is the user's active filter script. A user may have only one active script.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Import id in the form `<user_handle>:<name>`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *sieveScriptResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.sieveHost = clients.SieveHost
}

func (r *sieveScriptResource) connect(ctx context.Context, m *sieveScriptResourceModel) (*sieve.Client, error) {
	c, _, err := sieve.Dial(ctx, r.sieveHost)
	if err != nil {
		return nil, err
	}
	if err := c.Authenticate(m.UserHandle.ValueString(), m.Password.ValueString()); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("sieve: authenticate %s: %w", m.UserHandle.ValueString(), err)
	}
	return c, nil
}

func (r *sieveScriptResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sieveScriptResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.connect(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Unable to connect to ManageSieve", err.Error())
		return
	}
	defer func() { _ = c.Close() }()

	if err := c.PutScript(plan.Name.ValueString(), plan.Content.ValueString(), plan.Active.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Unable to upload Sieve script", err.Error())
		return
	}

	plan.ID = types.StringValue(plan.UserHandle.ValueString() + ":" + plan.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sieveScriptResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sieveScriptResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.Password.IsNull() {
		// Imported without a password: nothing to refresh against.
		resp.Diagnostics.AddAttributeWarning(
			path.Root("password"),
			"Sieve script not refreshed",
			"No password is available to connect to ManageSieve; state was not refreshed.",
		)
		return
	}

	c, err := r.connect(ctx, &state)
	if err != nil {
		resp.Diagnostics.AddError("Unable to connect to ManageSieve", err.Error())
		return
	}
	defer func() { _ = c.Close() }()

	scripts, err := c.ListScripts()
	if err != nil {
		resp.Diagnostics.AddError("Unable to list Sieve scripts", err.Error())
		return
	}

	found := false
	for _, script := range scripts {
		if script.Name != state.Name.ValueString() {
			continue
		}
		found = true
		state.Active = types.BoolValue(script.Active)
		break
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sieveScriptResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sieveScriptResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.connect(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Unable to connect to ManageSieve", err.Error())
		return
	}
	defer func() { _ = c.Close() }()

	// PUTSCRIPT overwrites an existing script of the same name.
	if err := c.PutScript(plan.Name.ValueString(), plan.Content.ValueString(), plan.Active.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Unable to upload Sieve script", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sieveScriptResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sieveScriptResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.connect(ctx, &state)
	if err != nil {
		resp.Diagnostics.AddError("Unable to connect to ManageSieve", err.Error())
		return
	}
	defer func() { _ = c.Close() }()

	if err := c.DeleteScript(state.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete Sieve script", err.Error())
	}
}

func (r *sieveScriptResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import sets user_handle/name from the "<user_handle>:<name>" id; the
	// password stays null, so refresh is skipped until a password is set.
	user, name, found := strings.Cut(req.ID, ":")
	if !found {
		resp.Diagnostics.AddError(
			"Invalid import id",
			fmt.Sprintf("Expected import id of the form <user_handle>:<name>, got %q.", req.ID),
		)
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_handle"), user)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}
