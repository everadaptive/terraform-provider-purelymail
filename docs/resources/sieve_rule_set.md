---
page_title: "purelymail_sieve_rule_set Resource - terraform-provider-purelymail"
subcategory: ""
---

# Resource: purelymail_sieve_rule_set

Structured Sieve filtering rules, compiled into a single Sieve script and
managed over the Manage Sieve protocol. A mailbox has only one active
script, so all rules must live in this one resource; use
[purelymail_sieve_script](sieve_script.md) for raw, hand-written scripts.

Note for OpenTofu 1.12+: nested blocks must use list-assignment syntax
(`rule = [{ ... }]`), not block syntax.

## Example Usage

```terraform
resource "purelymail_app_password" "sieve" {
  user_handle = purelymail_user.example.id
  name        = "terraform sieve"
}

resource "purelymail_sieve_rule_set" "main" {
  user_handle = purelymail_user.example.id
  password    = purelymail_app_password.sieve.app_password
  name        = "ruleset"
  active      = true

  rule = [
    {
      description = "file announce mail"
      match = [{
        header = "from"
        type   = "is"
        value  = "announce@example.com"
      }]
      action = [
        { type = "fileinto", target = "INBOX.announce" },
        { type = "stop" },
      ]
    },
    {
      description = "vacation reply"
      action = [{
        type    = "vacation"
        subject = "Out of office"
        message = "I am currently away."
        days    = 7
      }]
    },
  ]
}
```

## Argument Reference

- `user_handle` — (Required) Full username of the mailbox owner. Changing this forces a new resource.
- `password` — (Required, Sensitive) Mailbox password used to authenticate to ManageSieve. An app password is recommended. Changing this forces a new resource.
- `name` — (Required) Script name, e.g. `ruleset`. Changing this forces a new resource.
- `active` — (Optional/Computed) Whether the generated script is the user's active filter script. Defaults to `false`.
- `rule` — (Required) Ordered filter rules. Each rule runs in order until one matches and stops:
  - `description` — (Optional) Comment emitted above the rule.
  - `match` — (Optional) Conditions; multiple matches are ANDed together. Omit for an unconditional rule. Each has `header` (e.g. `from`, `subject`), `type` (`is`, `contains`, or `matches` glob), and `value`.
  - `action` — (Required) Ordered actions. Each has `type` — one of `fileinto`, `redirect`, `vacation`, `addflag`, `discard`, `stop` — plus:
    - `target` — target folder (`fileinto`) or address (`redirect`)
    - `subject`, `message`, `days` — `vacation` reply parameters
    - `flags` — IMAP flags for `addflag`, e.g. `["\\Seen"]`

## Attribute Reference

- `id` — `<user_handle>:<name>`.
- `content` — (Computed) The generated Sieve script (read-only view of what was uploaded).

## Import

Import by `<user_handle>:<name>`. The password is not known after import, so
refresh is skipped until a password is set in configuration. Imported rule
sets cannot reconstruct their structured `rule` list (the server cannot
return script content), so the configuration must match what was uploaded.

```shell
tofu import purelymail_sieve_rule_set.main hello@example.com:ruleset
```

## Interaction with purelymail_sieve_script

A mailbox has exactly one active script. Multiple Sieve resources for the
same user can coexist (each uploads a differently-named script), but at most
**one** resource for a given `user_handle` should set `active = true` — the
last one applied wins, and two resources both claiming active will
perpetually fight each other on every apply. Deletion always deactivates
first, so removing the active script never blocks the other one.

## Known server quirks

See [purelymail_sieve_script](sieve_script.md#known-server-quirks): script
content cannot be read back, so out-of-band edits to the generated script
are not detected.
