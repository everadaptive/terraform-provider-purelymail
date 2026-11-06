---
page_title: "purelymail_app_password Resource - terraform-provider-purelymail"
subcategory: ""
---

# Resource: purelymail_app_password

Manages an app password for a user.

The API cannot list app passwords, so the password is only known at creation
time and state refresh is not possible. Changing `name` or `user_handle`
creates a new app password.

## Example Usage

```terraform
resource "purelymail_app_password" "laptop" {
  user_handle = "hello@example.com"
  name        = "laptop"
}

output "app_password" {
  value     = purelymail_app_password.laptop.app_password
  sensitive = true
}
```

## Argument Reference

- `user_handle` — (Required) Full username of the mailbox owner, e.g. `user@example.com`. Changing this forces a new resource.
- `name` — (Optional) User-friendly description of the app password. Changing this forces a new resource.

## Attribute Reference

- `id` — Same as `app_password` (the password is the deletion key).
- `resolved_name` — (Computed) The final name registered with the server: the configured `name` (or `terraform`) plus a random suffix. App password names are not unique server-side and cannot be listed via the API, so the suffix makes each provider-created password identifiable in the web UI.
- `app_password` — (Computed, Sensitive) The generated app password.

## Import

Not supported: app passwords cannot be listed via the API, so import is
impossible. Re-create the resource instead (this generates a new password).
