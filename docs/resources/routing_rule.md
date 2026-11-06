---
page_title: "purelymail_routing_rule Resource - terraform-provider-purelymail"
subcategory: ""
---

# Resource: purelymail_routing_rule

Manages a routing (or catch-all) rule for a domain in your PurelyMail
account.

The API has no modify endpoint, so any change to the rule deletes and
re-creates it.

## Example Usage

```terraform
# Route mail for prefix "sales" to two addresses.
resource "purelymail_routing_rule" "sales" {
  domain_name      = purelymail_domain.example.name
  prefix           = true
  match_user       = "sales"
  target_addresses = ["alice@example.com", "bob@example.com"]
}

# Catch-all: deliver unmatched mail for the domain to one mailbox.
resource "purelymail_routing_rule" "catchall" {
  domain_name      = purelymail_domain.example.name
  prefix           = false
  match_user       = "*"
  target_addresses = ["hello@example.com"]
  catchall         = true
}
```

## Argument Reference

- `domain_name` — (Required) Domain the rule applies to, e.g. `example.com`. Changing this forces a new resource.
- `prefix` — (Required) Whether `match_user` matches address prefixes rather than exact local parts. Changing this forces a new resource.
- `match_user` — (Required) The local part of the user address to be matched, i.e. `user` in `user@domain.org`. Changing this forces a new resource.
- `target_addresses` — (Required) Full email addresses that matching mail is rerouted to. Changing this forces a new resource.
- `catchall` — (Optional) Whether this is a catch-all rule that will not fire if the address maps to a user. Defaults to `false`.

## Attribute Reference

- `id` — Server-assigned rule id.

## Import

Import by rule id (see `purelymail listRoutingRules` or the API):

```shell
tofu import purelymail_routing_rule.sales 12345
```
