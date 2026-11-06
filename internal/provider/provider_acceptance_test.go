package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func providerConfig() string {
	return fmt.Sprintf(`
provider "purelymail" {
  api_token = %q
}
`, os.Getenv("PURELYMAIL_API_TOKEN"))
}

func protoV6ProviderFactory() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"purelymail": func() (tfprotov6.ProviderServer, error) {
			return providerserver.NewProtocol6(New())(), nil
		},
	}
}

// precheck ensures acceptance tests only run with a real token.
func precheck(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set; skipping acceptance test")
	}
	if os.Getenv("PURELYMAIL_API_TOKEN") == "" {
		t.Fatal("PURELYMAIL_API_TOKEN must be set for acceptance tests")
	}
}

// These acceptance tests are read-only: the PurelyMail API rejects adding
// unverified domains ("DNS ownership checks did not pass"), so throwaway
// create/delete is not possible, and mutating real mailboxes is undesirable.
// Resource create/update/delete/import behavior is instead exercised live via
// the dev-override project (../cloudflare-dns), where existing domains and
// users are imported and a clean plan is the acceptance check. These tests
// cover the data-source read paths against the live API.

func TestAccDomainDataSource(t *testing.T) {
	precheck(t)
	domain := os.Getenv("PURELYMAIL_ACC_TEST_DOMAIN")
	if domain == "" {
		t.Skip("PURELYMAIL_ACC_TEST_DOMAIN not set; skipping")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + fmt.Sprintf(`
data "purelymail_domain" "test" {
  name = %q
}
`, domain),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.purelymail_domain.test", "name", domain),
					resource.TestCheckResourceAttr("data.purelymail_domain.test", "id", domain),
					resource.TestCheckResourceAttrSet("data.purelymail_domain.test", "passes_mx"),
					resource.TestCheckResourceAttrSet("data.purelymail_domain.test", "ownership_code"),
				),
			},
		},
	})
}

func TestAccDNSRecordsDataSource(t *testing.T) {
	precheck(t)
	domain := os.Getenv("PURELYMAIL_ACC_TEST_DOMAIN")
	if domain == "" {
		t.Skip("PURELYMAIL_ACC_TEST_DOMAIN not set; skipping")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + fmt.Sprintf(`
data "purelymail_dns_records" "test" {
  domain_name = %q
}
`, domain),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.purelymail_dns_records.test", "id", domain),
					resource.TestCheckResourceAttr("data.purelymail_dns_records.test", "records.1.type", "MX"),
					resource.TestCheckResourceAttr("data.purelymail_dns_records.test", "records.1.content", "mailserver.purelymail.com"),
					resource.TestCheckTypeSetElemNestedAttrs("data.purelymail_dns_records.test", "records.*", map[string]string{
						"type":    "CNAME",
						"content": "dmarcroot.purelymail.com",
					}),
				),
			},
		},
	})
}
