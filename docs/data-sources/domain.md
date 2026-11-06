---
page_title: "purelymail_domain Data Source - terraform-provider-purelymail"
subcategory: ""
---

# Data Source: purelymail_domain

Look up a domain in your PurelyMail account.

## Example Usage

```terraform
data "purelymail_domain" "example" {
  name = "example.com"
}
```

## Argument Reference

- `name` — (Required) Domain name, e.g. `example.com`.

## Attribute Reference

- `id` — Same as `name`.
- `allow_account_reset` — Whether the domain's account allows password reset.
- `symbolic_subaddressing` — Whether symbolic subaddressing (`+tag`) is enabled.
- `is_shared` — Whether the domain is shared to other accounts.
- `passes_mx`, `passes_spf`, `passes_dkim`, `passes_dmarc` — Results of PurelyMail's DNS checks.
- `ownership_code` — Full TXT record value to publish for domain ownership proof (e.g. as a Cloudflare DNS TXT record).
