---
page_title: "purelymail_domain Resource - terraform-provider-purelymail"
subcategory: ""
---

# Resource: purelymail_domain

Manages a domain in your PurelyMail account.

## Example Usage

```terraform
resource "purelymail_domain" "example" {
  name = "example.com"
}
```

## Argument Reference

- `name` — (Required) Domain name, e.g. `example.com`. Changing this forces a new resource.
- `allow_account_reset` — (Optional/Computed) Whether the domain's account allows password reset.
- `symbolic_subaddressing` — (Optional/Computed) Whether symbolic subaddressing (`+tag`) is enabled.

## Attribute Reference

- `id` — Same as `name`.
- `passes_mx`, `passes_spf`, `passes_dkim`, `passes_dmarc` — (Computed) Results of PurelyMail's DNS checks for the domain.

## Import

Import by domain name:

```shell
tofu import purelymail_domain.example example.com
```
