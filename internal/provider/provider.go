package provider

import (
	"context"
	"os"

	"github.com/everadaptive/terraform-provider-purelymail/internal/purelymail"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type purelymailProvider struct{}

// defaultSieveHost is PurelyMail's ManageSieve endpoint.
const defaultSieveHost = "imap.purelymail.com:4190"

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
			"sieve_host": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "ManageSieve endpoint for purelymail_sieve_script resources (defaults to `imap.purelymail.com:4190`).",
			},
		},
	}
}

// ProviderClients is the bundle passed to resources and data sources via
// Configure.
type ProviderClients struct {
	API *purelymail.Client
	// SieveHost is the ManageSieve endpoint (host:port).
	SieveHost string
}

func (p *purelymailProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config struct {
		APIToken  types.String `tfsdk:"api_token"`
		BaseURL   types.String `tfsdk:"base_url"`
		SieveHost types.String `tfsdk:"sieve_host"`
	}

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.APIToken.IsUnknown() {
		// Value not yet known (e.g. computed); the framework will re-run
		// Configure once it is.
		resp.Diagnostics.AddWarning(
			"PurelyMail API token not yet known",
			"The api_token value is not known yet; configuration will be retried.",
		)
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
		baseURL = "https://purelymail.com"
	}

	sieveHost := config.SieveHost.ValueString()
	if sieveHost == "" {
		sieveHost = defaultSieveHost
	}

	clients := &ProviderClients{
		API:       purelymail.New(baseURL, token),
		SieveHost: sieveHost,
	}

	resp.DataSourceData = clients
	resp.ResourceData = clients
}

func (p *purelymailProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDomainResource,
		NewUserResource,
		NewRoutingRuleResource,
		NewAppPasswordResource,
		NewPasswordResetResource,
		NewSieveScriptResource,
		NewSieveRuleSetResource,
	}
}

func (p *purelymailProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDomainDataSource,
		NewDNSRecordsDataSource,
		NewUsersDataSource,
		NewUserDataSource,
	}
}
