package provider

import (
	"context"
	"fmt"

	"github.com/everadaptive/terraform-provider-purelymail/internal/purelymail"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &domainResource{}
	_ resource.ResourceWithImportState = &domainResource{}
)

func NewDomainResource() resource.Resource {
	return &domainResource{}
}

type domainResource struct {
	client *purelymail.Client
}

type domainResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	AllowAccountReset     types.Bool   `tfsdk:"allow_account_reset"`
	SymbolicSubaddressing types.Bool   `tfsdk:"symbolic_subaddressing"`
	PassesMx              types.Bool   `tfsdk:"passes_mx"`
	PassesSpf             types.Bool   `tfsdk:"passes_spf"`
	PassesDkim            types.Bool   `tfsdk:"passes_dkim"`
	PassesDmarc           types.Bool   `tfsdk:"passes_dmarc"`
}

func (r *domainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *domainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a domain in your PurelyMail account.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Domain name, e.g. `example.com`.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Same as `name`; exists for framework compatibility.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"allow_account_reset": schema.BoolAttribute{
				MarkdownDescription: "Whether the domain's account allows password reset.",
				Computed:            true,
			},
			"symbolic_subaddressing": schema.BoolAttribute{
				MarkdownDescription: "Whether symbolic subaddressing (+tag) is enabled.",
				Computed:            true,
			},
			"passes_mx": schema.BoolAttribute{
				MarkdownDescription: "Whether the domain's MX records pass checks.",
				Computed:            true,
			},
			"passes_spf": schema.BoolAttribute{
				MarkdownDescription: "Whether the domain's SPF records pass checks.",
				Computed:            true,
			},
			"passes_dkim": schema.BoolAttribute{
				MarkdownDescription: "Whether the domain's DKIM records pass checks.",
				Computed:            true,
			},
			"passes_dmarc": schema.BoolAttribute{
				MarkdownDescription: "Whether the domain's DMARC records pass checks.",
				Computed:            true,
			},
		},
	}
}

func (r *domainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*purelymail.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected resource configure type",
			fmt.Sprintf("Expected *purelymail.Client, got: %T. Please report this issue.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *domainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan domainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.AddDomain(ctx, purelymail.AddDomainRequest{
		DomainName: plan.Name.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Unable to add domain", err.Error())
		return
	}

	if !r.read(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *domainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state domainResourceModel
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

// read refreshes the model from the API. It returns false if the domain no
// longer exists; API failures are added to diagnostics.
func (r *domainResource) read(ctx context.Context, m *domainResourceModel, diags *diag.Diagnostics) bool {
	dom, err := r.client.FindDomain(ctx, m.Name.ValueString())
	if err != nil {
		diags.AddError("Unable to read domain", err.Error())
		return false
	}
	if dom == nil {
		return false
	}

	m.ID = types.StringValue(dom.Name)
	m.AllowAccountReset = boolPtr(dom.AllowAccountReset)
	m.SymbolicSubaddressing = boolPtr(dom.SymbolicSubaddressing)
	if dom.DnsSummary != nil {
		m.PassesMx = boolPtr(dom.DnsSummary.PassesMx)
		m.PassesSpf = boolPtr(dom.DnsSummary.PassesSpf)
		m.PassesDkim = boolPtr(dom.DnsSummary.PassesDkim)
		m.PassesDmarc = boolPtr(dom.DnsSummary.PassesDmarc)
	}
	return true
}

func (r *domainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state domainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updates := purelymail.UpdateDomainSettingsRequest{
		Name: plan.Name.ValueString(),
	}
	if !plan.AllowAccountReset.Equal(state.AllowAccountReset) {
		v := plan.AllowAccountReset.ValueBool()
		updates.AllowAccountReset = &v
	}
	if !plan.SymbolicSubaddressing.Equal(state.SymbolicSubaddressing) {
		v := plan.SymbolicSubaddressing.ValueBool()
		updates.SymbolicSubaddressing = &v
	}

	// Only settings we can actually control are sent; the rest is read-only.
	if err := r.client.UpdateDomainSettings(ctx, updates); err != nil {
		resp.Diagnostics.AddError("Unable to update domain settings", err.Error())
		return
	}

	if !r.read(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *domainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state domainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteDomain(ctx, purelymail.DeleteDomainRequest{
		Name: state.Name.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Unable to delete domain", err.Error())
	}
}

func (r *domainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}
