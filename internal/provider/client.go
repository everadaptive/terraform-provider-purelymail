package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// client is the API client passed to resources and data sources via Configure.
// It is kept thin: typed request/response structs and error handling only.
type client struct {
	baseURL string
	token   string
}

func newClient(baseURL, token string) (*client, diag.Diagnostics) {
	var diags diag.Diagnostics

	if baseURL == "" {
		diags.AddError("Invalid provider configuration", "Base URL must not be empty")
		return nil, diags
	}
	if token == "" {
		diags.AddError("Invalid provider configuration", "API token must not be empty")
		return nil, diags
	}

	return &client{baseURL: baseURL, token: token}, diags
}
