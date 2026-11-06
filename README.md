# Terraform/OpenTofu Provider for PurelyMail

A [terraform-plugin-framework](https://developer.hashicorp.com/terraform/plugin/framework)-based
provider for [PurelyMail](https://purelymail.com), letting you manage domains,
users, DNS requirements, routing rules, app passwords, password reset methods,
and Sieve filter scripts declaratively.

- **Terraform Registry address:** `registry.opentofu.org/everadaptive/purelymail`
- **Go module:** `github.com/everadaptive/terraform-provider-purelymail`
- **Compatibility:** OpenTofu 1.x and Terraform 1.x (plugin protocol 6+)

## Quick Start

```terraform
terraform {
  required_providers {
    purelymail = {
      source = "everadaptive/purelymail"
    }
  }
}

provider "purelymail" {
  api_token = var.purelymail_api_token # or PURELYMAIL_API_TOKEN env var
}

resource "purelymail_domain" "example" {
  name = "example.com"
}

data "purelymail_dns_records" "example" {
  domain_name = purelymail_domain.example.name
}

resource "purelymail_user" "hello" {
  user_name   = "hello"
  domain_name = purelymail_domain.example.name
  password    = var.mailbox_password
}
```

## Resources

| Resource | Notes |
|---|---|
| `purelymail_domain` | Domain management, CRUD + import by name |
| `purelymail_user` | Mailbox users, CRUD + import by full username |
| `purelymail_routing_rule` | Routing/catch-all rules (no modify endpoint: changes recreate) |
| `purelymail_app_password` | Create/delete only — the API cannot list app passwords |
| `purelymail_password_reset` | Email/phone reset methods |
| `purelymail_sieve_script` | Raw Sieve scripts via ManageSieve (experimental) |
| `purelymail_sieve_rule_set` | Structured rules compiled to a Sieve script (experimental) |

## Data Sources

| Data Source | Notes |
|---|---|
| `purelymail_domain` | Domain settings, DNS check results, ownership code |
| `purelymail_dns_records` | The standard record set PurelyMail requires (ownership TXT, MX, SPF, DKIM, DMARC, autoconfig) — use these to create matching records at your DNS provider |

Full argument references live in `docs/`, with examples under `examples/`.

## Provider Configuration

| Attribute | Environment | Description |
|---|---|---|
| `api_token` | `PURELYMAIL_API_TOKEN` | PurelyMail API token (sent as `Purelymail-Api-Token`) |
| `base_url` | — | Override API base URL (testing) |
| `sieve_host` | — | ManageSieve endpoint, default `imap.purelymail.com:4190` |

Note: Sieve resources authenticate with the **mailbox password** (an app
password is ideal), not the API token — ManageSieve is a separate protocol
from the HTTP API.

## Development

```shell
go build ./... && go test ./...   # unit tests
go build -o bin/terraform-provider-purelymail .
```

- `golangci-lint run` must pass.
- Acceptance tests: `TF_ACC=1 PURELYMAIL_API_TOKEN=... go test ./internal/provider/ -run TestAcc`
  (read-only against the live API; see `internal/provider/provider_acceptance_test.go`).
- The sibling project `../cloudflare-dns` doubles as the live integration
  harness: `~/.tofurc` dev-overrides this provider's `bin/` directory, and a
  clean `tofu plan` there is the acceptance check.
- API surface is confirmed against the official spec at
  <https://news.purelymail.com/api/index.html> (`swagger-spec.js`).

See `AGENTS.md` for contributor/agent conventions (docs and tests ship with
every change).

## Known Limitations

- The PurelyMail API cannot list app passwords or return user passwords;
  those resources have intentionally limited refresh semantics (documented
  per resource).
- Account-level Sieve filters are portal-only (no API endpoint) and cannot
  be managed by this provider.
- The server's ManageSieve implementation does not support `GETSCRIPT`, so
  script content drift is undetectable.
