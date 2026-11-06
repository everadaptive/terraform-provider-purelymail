---
page_title: "Provider: purelymail"
subcategory: ""
page_title: "PurelyMail Provider"
---

# PurelyMail Provider

Manage [PurelyMail](https://purelymail.com) email hosting (domains, users, DNS
requirements) declaratively with OpenTofu/Terraform.

## Example Usage

```terraform
provider "purelymail" {
  api_token = var.purelymail_api_token
}
```

## Argument Reference

- `api_token` — (Optional) PurelyMail API token. May also be provided via the
  `PURELYMAIL_API_TOKEN` environment variable. The token is sent as the
  `Purelymail-Api-Token` header.
- `base_url` — (Optional) Override the API base URL (defaults to
  `https://purelymail.com`). Useful for testing.

## Generating an API token

Create an API token in the PurelyMail settings UI (Account → API keys).
