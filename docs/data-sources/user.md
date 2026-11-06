---
page_title: "purelymail_user Data Source - terraform-provider-purelymail"
subcategory: ""
---

# Data Source: purelymail_user

Look up a mailbox user in your PurelyMail account. Note: the API does not
return whether password reset is enabled.

## Example Usage

```terraform
# By parts...
data "purelymail_user" "by_parts" {
  user_name   = "hello"
  domain_name = purelymail_domain.example.name
}

# ...or by full username.
data "purelymail_user" "by_id" {
  id = "hello@example.com"
}
```

## Argument Reference

- `user_name` — (Optional) Local part of the username.
- `domain_name` — (Optional) Domain of the mailbox.
- `id` — (Optional/Computed) Full username (`user_name@domain_name`); may be used as the lookup key instead of the two attributes above.

## Attribute Reference

- `id` — Full username.
- `enable_search_indexing` — Whether the mailbox is indexed for search.
- `require_two_factor_authentication` — Whether two-factor authentication is required.
- `enable_spam_filtering` — Whether spam filtering is enabled.
