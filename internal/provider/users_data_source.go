package provider

import (
	"context"
	"fmt"

	"github.com/everadaptive/terraform-provider-purelymail/internal/purelymail"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &usersDataSource{}

func NewUsersDataSource() datasource.DataSource {
	return &usersDataSource{}
}

type usersDataSource struct {
	client *purelymail.Client
}

type usersDataSourceModel struct {
	ID    types.String   `tfsdk:"id"`
	Users []types.String `tfsdk:"users"`
}

func (d *usersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_users"
}

func (d *usersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "All usernames (mailboxes) under your PurelyMail account, " +
			"e.g. `user@example.com`. Useful for lookups and referencing existing mailboxes.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Constant identifier for this data source.",
				Computed:            true,
			},
			"users": schema.ListAttribute{
				MarkdownDescription: "Full usernames of all mailboxes (up to 1000).",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *usersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *usersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model usersDataSourceModel

	users, err := d.client.ListUser(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list users", err.Error())
		return
	}

	values := make([]types.String, len(users))
	for i, u := range users {
		values[i] = types.StringValue(u)
	}
	model.Users = values
	model.ID = types.StringValue("users")

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
