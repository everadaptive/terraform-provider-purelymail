package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/everadaptive/terraform-provider-purelymail/internal/purelymail"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithImportState = &routingRuleResource{}

func NewRoutingRuleResource() resource.Resource {
	return &routingRuleResource{}
}

type routingRuleResource struct {
	client *purelymail.Client
}

type routingRuleResourceModel struct {
	ID              types.String `tfsdk:"id"`
	DomainName      types.String `tfsdk:"domain_name"`
	Prefix          types.Bool   `tfsdk:"prefix"`
	MatchUser       types.String `tfsdk:"match_user"`
	TargetAddresses types.List   `tfsdk:"target_addresses"`
	Catchall        types.Bool   `tfsdk:"catchall"`
}

func (r *routingRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_routing_rule"
}

func (r *routingRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	// The API has no modify endpoint: any change deletes and re-creates the
	// rule, so every attribute forces replacement.
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a routing (or catch-all) rule for a domain in your " +
			"PurelyMail account. Any change re-creates the rule.",
		Attributes: map[string]schema.Attribute{
			"domain_name": schema.StringAttribute{
				MarkdownDescription: "Domain the rule applies to, e.g. `example.com`.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"prefix": schema.BoolAttribute{
				MarkdownDescription: "Whether `match_user` matches address prefixes rather than exact local parts.",
				Required:            true,
			},
			"match_user": schema.StringAttribute{
				MarkdownDescription: "The local part of the user address to be matched, i.e. `user` in `user@domain.org`.",
				Required:            true,
			},
			"target_addresses": schema.ListAttribute{
				MarkdownDescription: "Full email addresses that matching mail is rerouted to.",
				Required:            true,
				ElementType:         types.StringType,
			},
			"catchall": schema.BoolAttribute{
				MarkdownDescription: "Whether this is a catch-all rule that will not fire if the address maps to a user.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Server-assigned rule id.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *routingRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *routingRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan routingRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API does not return the new rule's id, so locate it after creation
	// by diffing the rule list.
	before, err := r.client.ListRoutingRules(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list routing rules", err.Error())
		return
	}

	if err := r.client.CreateRoutingRule(ctx, r.request(ctx, &plan)); err != nil {
		resp.Diagnostics.AddError("Unable to create routing rule", err.Error())
		return
	}

	after, err := r.client.ListRoutingRules(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list routing rules", err.Error())
		return
	}

	created := findNewRule(before, after, &plan)
	if created == nil {
		resp.Diagnostics.AddError(
			"Routing rule not found after create",
			"The rule was created but could not be located in the account's rule list.",
		)
		return
	}

	r.fromAPI(ctx, created, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// findNewRule locates the rule matching the plan among rules that did not
// exist before the create call.
func findNewRule(before, after []purelymail.RoutingRule, plan *routingRuleResourceModel) *purelymail.RoutingRule {
	existing := make(map[int64]bool, len(before))
	for _, r := range before {
		existing[r.ID] = true
	}
	for i := range after {
		r := &after[i]
		if !existing[r.ID] &&
			r.DomainName == plan.DomainName.ValueString() &&
			r.Prefix == plan.Prefix.ValueBool() &&
			r.MatchUser == plan.MatchUser.ValueString() &&
			r.Catchall == plan.Catchall.ValueBool() {
			return r
		}
	}
	return nil
}

func (r *routingRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state routingRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid rule id", err.Error())
		return
	}

	rule, err := r.client.FindRoutingRule(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read routing rule", err.Error())
		return
	}
	if rule == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.fromAPI(ctx, rule, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *routingRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Every attribute forces replacement, so Update is never reached.
	resp.Diagnostics.AddError(
		"Unexpected update",
		"purelymail_routing_rule does not support in-place updates; all attributes force replacement. Please report this issue.",
	)
}

func (r *routingRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state routingRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid rule id", err.Error())
		return
	}

	if err := r.client.DeleteRoutingRule(ctx, purelymail.DeleteRoutingRuleRequest{RoutingRuleID: id}); err != nil {
		resp.Diagnostics.AddError("Unable to delete routing rule", err.Error())
	}
}

func (r *routingRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *routingRuleResource) request(ctx context.Context, m *routingRuleResourceModel) purelymail.CreateRoutingRuleRequest {
	var targets []string
	diags := m.TargetAddresses.ElementsAs(ctx, &targets, false)
	if diags.HasError() {
		return purelymail.CreateRoutingRuleRequest{}
	}

	return purelymail.CreateRoutingRuleRequest{
		DomainName:      m.DomainName.ValueString(),
		Prefix:          m.Prefix.ValueBool(),
		MatchUser:       m.MatchUser.ValueString(),
		TargetAddresses: targets,
		Catchall:        m.Catchall.ValueBool(),
	}
}

func (r *routingRuleResource) fromAPI(ctx context.Context, rule *purelymail.RoutingRule, m *routingRuleResourceModel) {
	m.ID = types.StringValue(strconv.FormatInt(rule.ID, 10))
	m.DomainName = types.StringValue(rule.DomainName)
	m.Prefix = types.BoolValue(rule.Prefix)
	m.MatchUser = types.StringValue(rule.MatchUser)
	m.Catchall = types.BoolValue(rule.Catchall)

	list, diags := types.ListValueFrom(ctx, types.StringType, rule.TargetAddresses)
	if diags.HasError() {
		m.TargetAddresses = types.ListNull(types.StringType)
		return
	}
	m.TargetAddresses = list
}
