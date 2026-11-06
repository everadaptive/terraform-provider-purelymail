package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/everadaptive/terraform-provider-purelymail/internal/sieve"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithImportState = &sieveRuleSetResource{}

func NewSieveRuleSetResource() resource.Resource {
	return &sieveRuleSetResource{}
}

type sieveRuleSetResource struct {
	sieveHost string
}

func (r *sieveRuleSetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sieve_rule_set"
}

func (r *sieveRuleSetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Structured Sieve filtering rules, compiled into a single Sieve " +
			"script and managed over the Manage Sieve protocol. A mailbox has only " +
			"one active script, so all rules must live in this one resource; use " +
			"`purelymail_sieve_script` for raw, hand-written scripts. Script content " +
			"cannot be read back from the server, so out-of-band edits are not detected.",
		Attributes: map[string]schema.Attribute{
			"user_handle": schema.StringAttribute{
				MarkdownDescription: "Full username of the mailbox owner, e.g. `user@example.com`.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Mailbox password used to authenticate to ManageSieve. An app password works and is recommended.",
				Required:            true,
				Sensitive:           true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Script name, e.g. `main`. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"active": schema.BoolAttribute{
				MarkdownDescription: "Whether the generated script is the user's active filter script.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"rule": schema.ListNestedAttribute{
				MarkdownDescription: "Ordered filter rules. Each rule runs in order until one matches and stops.",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"description": schema.StringAttribute{
							MarkdownDescription: "Comment emitted above the rule.",
							Optional:            true,
						},
						"match": schema.ListNestedAttribute{
							MarkdownDescription: "Conditions; multiple matches are ANDed together. Omit for an unconditional rule.",
							Optional:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"header": schema.StringAttribute{
										MarkdownDescription: "Message header to test, e.g. `from`, `subject`.",
										Required:            true,
									},
									"type": schema.StringAttribute{
										MarkdownDescription: "Comparison: `is`, `contains`, or `matches` (glob).",
										Required:            true,
									},
									"value": schema.StringAttribute{
										MarkdownDescription: "Value to compare against.",
										Required:            true,
									},
								},
							},
						},
						"action": schema.ListNestedAttribute{
							MarkdownDescription: "Ordered actions to run when the rule matches.",
							Required:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"type": schema.StringAttribute{
										MarkdownDescription: "One of `fileinto`, `redirect`, `vacation`, `addflag`, `discard`, `stop`.",
										Required:            true,
									},
									"target": schema.StringAttribute{
										MarkdownDescription: "Target folder (for `fileinto`) or address (for `redirect`).",
										Optional:            true,
									},
									"subject": schema.StringAttribute{
										MarkdownDescription: "Subject for `vacation` replies.",
										Optional:            true,
									},
									"message": schema.StringAttribute{
										MarkdownDescription: "Message body for `vacation` replies.",
										Optional:            true,
									},
									"days": schema.Int64Attribute{
										MarkdownDescription: "Minimum days between `vacation` replies.",
										Optional:            true,
									},
									"flags": schema.ListAttribute{
										MarkdownDescription: "IMAP flags for `addflag`, e.g. `[\"\\\\Seen\"]`.",
										Optional:            true,
										ElementType:         types.StringType,
									},
								},
							},
						},
					},
				},
			},
			"content": schema.StringAttribute{
				MarkdownDescription: "The generated Sieve script (read-only view of what was uploaded).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "`<user_handle>:<name>`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *sieveRuleSetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *sieveRuleSetResource) connect(ctx context.Context, m *sieveRuleSetModel) (*sieve.Client, error) {
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

func (r *sieveRuleSetResource) upload(ctx context.Context, m *sieveRuleSetModel, diags *diag.Diagnostics) {
	content, err := generateSieve(m.Rules)
	if err != nil {
		diags.AddError("Unable to generate Sieve script", err.Error())
		return
	}

	c, err := r.connect(ctx, m)
	if err != nil {
		diags.AddError("Unable to connect to ManageSieve", err.Error())
		return
	}
	defer c.Close()

	if err := c.PutScript(m.Name.ValueString(), content, m.Active.ValueBool()); err != nil {
		diags.AddError("Unable to upload Sieve script", err.Error())
		return
	}

	m.Content = types.StringValue(content)
	m.ID = types.StringValue(m.UserHandle.ValueString() + ":" + m.Name.ValueString())
}

func (r *sieveRuleSetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sieveRuleSetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upload(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sieveRuleSetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sieveRuleSetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.connect(ctx, &state)
	if err != nil {
		resp.Diagnostics.AddError("Unable to connect to ManageSieve", err.Error())
		return
	}
	defer c.Close()

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

func (r *sieveRuleSetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sieveRuleSetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upload(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sieveRuleSetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sieveRuleSetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.connect(ctx, &state)
	if err != nil {
		resp.Diagnostics.AddError("Unable to connect to ManageSieve", err.Error())
		return
	}
	defer c.Close()

	if err := c.DeleteScript(state.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete Sieve script", err.Error())
	}
}

func (r *sieveRuleSetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import sets user_handle/name from the "<user_handle>:<name>" id; the
	// password stays unset, so refresh is skipped until a password is set.
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
