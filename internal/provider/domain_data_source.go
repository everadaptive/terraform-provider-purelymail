package provider

import (
	"context"
	"fmt"

	"github.com/everadaptive/terraform-provider-purelymail/internal/purelymail"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &domainDataSource{}

func NewDomainDataSource() datasource.DataSource {
	return &domainDataSource{}
}

type domainDataSource struct {
	client *purelymail.Client
}

type domainDataSourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	AllowAccountReset     types.Bool   `tfsdk:"allow_account_reset"`
	SymbolicSubaddressing types.Bool   `tfsdk:"symbolic_subaddressing"`
	IsShared              types.Bool   `tfsdk:"is_shared"`
	PassesMx              types.Bool   `tfsdk:"passes_mx"`
	PassesSpf             types.Bool   `tfsdk:"passes_spf"`
	PassesDkim            types.Bool   `tfsdk:"passes_dkim"`
	PassesDmarc           types.Bool   `tfsdk:"passes_dmarc"`
	OwnershipCode         types.String `tfsdk:"ownership_code"`
}

func (d *domainDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (d *domainDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up a domain in your PurelyMail account.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Domain name, e.g. `example.com`.",
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Same as `name`; exists for framework compatibility.",
				Computed:            true,
			},
			"allow_account_reset": schema.BoolAttribute{
				MarkdownDescription: "Whether the domain's account allows password reset.",
				Computed:            true,
			},
			"symbolic_subaddressing": schema.BoolAttribute{
				MarkdownDescription: "Whether symbolic subaddressing (+tag) is enabled.",
				Computed:            true,
			},
			"is_shared": schema.BoolAttribute{
				MarkdownDescription: "Whether the domain is shared to other accounts.",
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
			"ownership_code": schema.StringAttribute{
				MarkdownDescription: "Full TXT record value to publish for domain ownership proof " +
					"(e.g. as a Cloudflare DNS TXT record), fetched from the API.",
				Computed: true,
			},
		},
	}
}

func (d *domainDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*purelymail.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected data source configure type",
			fmt.Sprintf("Expected *purelymail.Client, got: %T. Please report this issue.", req.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *domainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model domainDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domains, err := d.client.ListDomains(ctx, purelymail.ListDomainsRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Unable to list domains", err.Error())
		return
	}

	for _, dom := range domains {
		if dom.Name != model.Name.ValueString() {
			continue
		}

		model.ID = types.StringValue(dom.Name)
		model.AllowAccountReset = boolPtr(dom.AllowAccountReset)
		model.SymbolicSubaddressing = boolPtr(dom.SymbolicSubaddressing)
		model.IsShared = boolPtr(dom.IsShared)
		if dom.DnsSummary != nil {
			model.PassesMx = boolPtr(dom.DnsSummary.PassesMx)
			model.PassesSpf = boolPtr(dom.DnsSummary.PassesSpf)
			model.PassesDkim = boolPtr(dom.DnsSummary.PassesDkim)
			model.PassesDmarc = boolPtr(dom.DnsSummary.PassesDmarc)
		}

		code, err := d.client.GetOwnershipCode(ctx, purelymail.GetOwnershipCodeRequest{
			DomainName: dom.Name,
		})
		if err != nil {
			resp.Diagnostics.AddError("Unable to get ownership code", err.Error())
			return
		}
		model.OwnershipCode = types.StringValue(code.Code)

		resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
		return
	}

	resp.Diagnostics.AddError(
		"Domain not found",
		fmt.Sprintf("No domain named %q exists in this account.", model.Name.ValueString()),
	)
}

func boolPtr(v *bool) types.Bool {
	if v == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*v)
}
