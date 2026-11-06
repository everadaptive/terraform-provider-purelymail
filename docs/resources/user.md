---
page_title: "purelymail_user Resource - terraform-provider-purelymail"
subcategory: ""
---

# Resource: purelymail_user

Manages a mailbox user in your PurelyMail account.

## Example Usage

```terraform
resource "purelymail_user" "example" {
  user_name   = "hello"
  domain_name = purelymail_domain.example.name
  password    = var.mailbox_password
}
```

## Argument Reference

- `user_name` — (Required) Local part of the username, e.g. `user` in `user@example.com`. Changing this forces a new resource.
- `domain_name` — (Required) Domain of the mailbox. Changing this forces a new resource.
- `password` — (Optional, Sensitive) Mailbox password. Required on create; on import it stays null and is only sent when changed (the API never returns passwords).
- `enable_password_reset` — (Optional) Whether password reset methods may be used. Write-only: the API does not return this flag.
- `enable_search_indexing` — (Optional/Computed) Whether the mailbox is indexed for search.
- `require_two_factor_authentication` — (Optional/Computed) Whether two-factor authentication is required.

## Attribute Reference

- `id` — Full username (`user_name@domain_name`).
- `enable_spam_filtering` — (Computed) Whether spam filtering is enabled.

## Import

Import by full username:

```shell
tofu import purelymail_user.example hello@example.com
```
