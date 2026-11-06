package provider

import (
	"context"
	"fmt"

	"github.com/everadaptive/terraform-provider-purelymail/internal/purelymail"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &dnsRecordsDataSource{}

func NewDNSRecordsDataSource() datasource.DataSource {
	return &dnsRecordsDataSource{}
}

type dnsRecordsDataSource struct {
	client *purelymail.Client
}

type dnsRecordModel struct {
	Type     types.String `tfsdk:"type"`
	Name     types.String `tfsdk:"name"`
	Content  types.String `tfsdk:"content"`
	Priority types.Int64  `tfsdk:"priority"`
}

type dnsRecordsDataSourceModel struct {
	ID           types.String     `tfsdk:"id"`
	DomainName   types.String     `tfsdk:"domain_name"`
	OwnershipTxt types.String     `tfsdk:"ownership_txt"`
	Records      []dnsRecordModel `tfsdk:"records"`
}

func (d *dnsRecordsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_records"
}

func (d *dnsRecordsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The standard DNS record set PurelyMail requires for a domain " +
			"(MX, SPF, DKIM, DMARC, autoconfig, and the ownership-proof TXT). " +
			"Use these values to create matching records at your DNS provider.",
		Attributes: map[string]schema.Attribute{
			"domain_name": schema.StringAttribute{
				MarkdownDescription: "Domain name, e.g. `example.com`.",
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Same as `domain_name`; exists for framework compatibility.",
				Computed:            true,
			},
			"ownership_txt": schema.StringAttribute{
				MarkdownDescription: "Full TXT record value for ownership proof, also available " +
					"as the first element of `records`.",
				Computed: true,
			},
			"records": schema.ListNestedAttribute{
				MarkdownDescription: "All records required by PurelyMail for this domain.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							MarkdownDescription: "DNS record type (TXT, MX, CNAME, SRV).",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "Record name (relative to the domain, or the domain itself for apex records).",
							Computed:            true,
						},
						"content": schema.StringAttribute{
							MarkdownDescription: "Record content/target.",
							Computed:            true,
						},
						"priority": schema.Int64Attribute{
							MarkdownDescription: "Priority (MX/SRV records only, otherwise 0).",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *dnsRecordsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	clients, ok := req.ProviderData.(*ProviderClients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected data source configure type",
			fmt.Sprintf("Expected *ProviderClients, got: %T. Please report this issue.", req.ProviderData),
		)
		return
	}
	d.client = clients.API
}

func (d *dnsRecordsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model dnsRecordsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain := model.DomainName.ValueString()

	code, err := d.client.GetOwnershipCode(ctx, purelymail.GetOwnershipCodeRequest{DomainName: domain})
	if err != nil {
		resp.Diagnostics.AddError("Unable to get ownership code", err.Error())
		return
	}

	// The record set below is what PurelyMail documents for incoming mail
	// delivery; the ownership code is the only per-domain dynamic value.
	records := []dnsRecordModel{
		{Type: types.StringValue("TXT"), Name: types.StringValue(domain), Content: types.StringValue(code.Code), Priority: types.Int64Value(0)},
		{Type: types.StringValue("MX"), Name: types.StringValue(domain), Content: types.StringValue("mailserver.purelymail.com"), Priority: types.Int64Value(50)},
		{Type: types.StringValue("TXT"), Name: types.StringValue(domain), Content: types.StringValue(`"v=spf1 include:_spf.purelymail.com ~all"`), Priority: types.Int64Value(0)},
		{Type: types.StringValue("CNAME"), Name: types.StringValue("purelymail1._domainkey." + domain), Content: types.StringValue("key1.dkimroot.purelymail.com"), Priority: types.Int64Value(0)},
		{Type: types.StringValue("CNAME"), Name: types.StringValue("purelymail2._domainkey." + domain), Content: types.StringValue("key2.dkimroot.purelymail.com"), Priority: types.Int64Value(0)},
		{Type: types.StringValue("CNAME"), Name: types.StringValue("purelymail3._domainkey." + domain), Content: types.StringValue("key3.dkimroot.purelymail.com"), Priority: types.Int64Value(0)},
		{Type: types.StringValue("CNAME"), Name: types.StringValue("_dmarc." + domain), Content: types.StringValue("dmarcroot.purelymail.com"), Priority: types.Int64Value(0)},
		{Type: types.StringValue("CNAME"), Name: types.StringValue("autoconfig." + domain), Content: types.StringValue("autoconfig.purelymail.com"), Priority: types.Int64Value(0)},
		{Type: types.StringValue("SRV"), Name: types.StringValue("_autodiscover._tcp." + domain), Content: types.StringValue("autodiscover.purelymail.com"), Priority: types.Int64Value(0)},
	}

	model.ID = types.StringValue(domain)
	model.OwnershipTxt = types.StringValue(code.Code)
	model.Records = records

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
