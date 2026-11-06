---
page_title: "purelymail_sieve_script Resource - terraform-provider-purelymail"
subcategory: ""
---

# Resource: purelymail_sieve_script

Manages a Sieve filter script for a user via the [Manage Sieve protocol
(RFC 5804)](https://datatracker.ietf.org/doc/html/rfc5804). PurelyMail exposes
ManageSieve at `imap.purelymail.com:4190` (STARTTLS, SASL PLAIN), so this
resource authenticates with the **mailbox password** — an
[purelymail_app_password](app_password.md) is ideal — not the API token.

## Example Usage

```terraform
resource "purelymail_app_password" "sieve" {
  user_handle = purelymail_user.example.id
  name        = "terraform sieve"
}

resource "purelymail_sieve_script" "main" {
  user_handle = purelymail_user.example.id
  password    = purelymail_app_password.sieve.app_password
  name        = "main"
  content     = <<-EOT
    require ["fileinto"];
    if header :contains "subject" "[news]" { fileinto "INBOX.news"; stop; }
  EOT
  active      = true
}
```

## Argument Reference

- `user_handle` — (Required) Full username of the mailbox owner, e.g. `user@example.com`. Changing this forces a new resource.
- `password` — (Required, Sensitive) Mailbox password used to authenticate to ManageSieve. Changing this forces a new resource.
- `name` — (Required) Script name, e.g. `main`. Changing this forces a new resource.
- `content` — (Required) Sieve script source.
- `active` — (Optional/Computed) Whether the script is the user's active filter script. Defaults to `false`. A user may have only one active script.

## Attribute Reference

- `id` — `<user_handle>:<name>`.

## Import

Import by `<user_handle>:<name>`. The password is not known after import, so
refresh is skipped until a password is set in configuration.

```shell
tofu import purelymail_sieve_script.main hello@example.com:main
```

## Interaction with purelymail_sieve_rule_set

A mailbox has exactly one active script. Multiple Sieve resources for the
same user can coexist (each uploads a differently-named script), but at most
**one** resource for a given `user_handle` should set `active = true` — the
last one applied wins, and two resources both claiming active will
perpetually fight each other on every apply. Deletion always deactivates
first, so removing the active script never blocks the other one.

## Known server quirks

PurelyMail's ManageSieve implementation (Apache ManageSieve v1.0) has some
limitations the provider works around:

- `GETSCRIPT` returns an error, so script **content cannot be read back** and
  drift on `content` is undetectable.
- `LISTSCRIPTS` output may start with a malformed spurious entry; the
  provider skips it. All real scripts (active or not) are listed, so
  out-of-band deletion is detected on refresh.
- Deleting the active script requires deactivating it first; the provider
  handles this automatically.
