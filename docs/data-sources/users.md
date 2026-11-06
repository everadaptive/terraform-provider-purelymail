---
page_title: "purelymail_users Data Source - terraform-provider-purelymail"
subcategory: ""
---

# Data Source: purelymail_users

All usernames (mailboxes) under your PurelyMail account, e.g.
`user@example.com`. Useful for lookups and referencing existing mailboxes.

## Example Usage

```terraform
data "purelymail_users" "all" {}

output "mailbox_count" {
  value = length(data.purelymail_users.all.users)
}
```

## Attribute Reference

- `id` — Constant identifier (`users`).
- `users` — List of full usernames of all mailboxes (up to 1000).
