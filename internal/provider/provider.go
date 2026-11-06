package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type purelymailProvider struct{}

func New() provider.Provider {
	return &purelymailProvider{}
}

func (p *purelymailProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "purelymail"
}

func (p *purelymailProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "PurelyMail API token. May also be provided via the " +
					"`PURELYMAIL_API_TOKEN` environment variable.",
			},
			"base_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Base URL of the PurelyMail API. Useful for testing.",
			},
		},
	}
}

type providerData struct {
	APIToken string
	BaseURL  string
}

func (p *purelymailProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config struct {
		APIToken types.String `tfsdk:"api_token"`
		BaseURL  types.String `tfsdk:"base_url"`
	}

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token := config.APIToken.ValueString()
	if token == "" {
		token = os.Getenv("PURELYMAIL_API_TOKEN")
	}
	if token == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Missing PurelyMail API token",
			"Set the api_token provider attribute or the PURELYMAIL_API_TOKEN environment variable.",
		)
		return
	}

	baseURL := config.BaseURL.ValueString()
	if baseURL == "" {
		baseURL = "https://api.purelymail.com"
	}

	client, diags := newClient(baseURL, token)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *purelymailProvider) Resources(_ context.Context) []func() resource.Resource {
	return nil
}

func (p *purelymailProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
