package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/everadaptive/terraform-provider-purelymail/internal/purelymail"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &userDataSource{}

func NewUserDataSource() datasource.DataSource {
	return &userDataSource{}
}

type userDataSource struct {
	client *purelymail.Client
}

type userDataSourceModel struct {
	ID                             types.String `tfsdk:"id"`
	UserName                       types.String `tfsdk:"user_name"`
	DomainName                     types.String `tfsdk:"domain_name"`
	EnableSearchIndexing           types.Bool   `tfsdk:"enable_search_indexing"`
	RequireTwoFactorAuthentication types.Bool   `tfsdk:"require_two_factor_authentication"`
	EnableSpamFiltering            types.Bool   `tfsdk:"enable_spam_filtering"`
}

func (d *userDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *userDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up a mailbox user in your PurelyMail account. Note: the " +
			"API does not return whether password reset is enabled.",
		Attributes: map[string]schema.Attribute{
			"user_name": schema.StringAttribute{
				MarkdownDescription: "Local part of the username, e.g. `user` in `user@example.com`. " +
					"Alternatively, supply the full username in `id`.",
				Optional: true,
			},
			"domain_name": schema.StringAttribute{
				MarkdownDescription: "Domain of the mailbox, e.g. `example.com`. Required unless the " +
					"full username is supplied via `id`.",
				Optional: true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Full username (`user_name@domain_name`). May be used as the lookup key.",
				Optional:            true,
				Computed:            true,
			},
			"enable_search_indexing": schema.BoolAttribute{
				MarkdownDescription: "Whether the mailbox is indexed for search.",
				Computed:            true,
			},
			"require_two_factor_authentication": schema.BoolAttribute{
				MarkdownDescription: "Whether two-factor authentication is required.",
				Computed:            true,
			},
			"enable_spam_filtering": schema.BoolAttribute{
				MarkdownDescription: "Whether spam filtering is enabled.",
				Computed:            true,
			},
		},
	}
}

func (d *userDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *userDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model userDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Allow lookup by full username via `id`, or by user_name + domain_name.
	if model.UserName.IsNull() && !model.ID.IsNull() {
		local, domain, found := strings.Cut(model.ID.ValueString(), "@")
		if !found {
			resp.Diagnostics.AddError(
				"Invalid id",
				fmt.Sprintf("Expected id of the form user@domain, got %q.", model.ID.ValueString()),
			)
			return
		}
		model.UserName = types.StringValue(local)
		model.DomainName = types.StringValue(domain)
	}
	if model.UserName.IsNull() || model.DomainName.IsNull() {
		resp.Diagnostics.AddError(
			"Missing lookup key",
			"Provide either user_name and domain_name, or id (full username).",
		)
		return
	}

	full := model.UserName.ValueString() + "@" + model.DomainName.ValueString()
	user, err := d.client.GetUser(ctx, purelymail.GetUserRequest{UserName: full})
	if err != nil {
		resp.Diagnostics.AddError("Unable to read user", err.Error())
		return
	}

	model.ID = types.StringValue(full)
	model.EnableSearchIndexing = boolPtr(user.EnableSearchIndexing)
	model.RequireTwoFactorAuthentication = boolPtr(user.RequireTwoFactorAuthentication)
	model.EnableSpamFiltering = boolPtr(user.EnableSpamFiltering)

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
