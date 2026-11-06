---
page_title: "purelymail_password_reset Resource - terraform-provider-purelymail"
subcategory: ""
---

# Resource: purelymail_password_reset

Manages a password reset method (email or phone) for a user.

## Example Usage

```terraform
resource "purelymail_password_reset" "backup" {
  user_name   = "hello@example.com"
  type        = "email"
  target      = "recovery@example.net"
  description = "Backup email"
}
```

## Argument Reference

- `user_name` — (Required) Full username of the mailbox owner, e.g. `user@example.com`. Changing this forces a new resource.
- `type` — (Required) Type of reset method, either `email` or `phone`. Changing this forces a new resource.
- `target` — (Required) Email address or phone number the reset is sent to.
- `description` — (Optional/Computed) Human-readable description of the method.
- `allow_mfa_reset` — (Optional/Computed) Whether this method may also reset the user's MFA. Defaults to `true`.

## Attribute Reference

- `id` — `<user_name>:<target>`.

## Import

Import by `<user_name>:<target>`:

```shell
tofu import purelymail_password_reset.backup hello@example.com:recovery@example.net
```
