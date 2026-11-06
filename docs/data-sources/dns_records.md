---
page_title: "purelymail_dns_records Data Source - terraform-provider-purelymail"
subcategory: ""
---

# Data Source: purelymail_dns_records

The standard DNS record set PurelyMail requires for a domain (MX, SPF, DKIM,
DMARC, autoconfig, and the ownership-proof TXT). Use these values to create
matching records at your DNS provider, e.g. Cloudflare.

## Example Usage

```terraform
data "purelymail_dns_records" "example" {
  domain_name = purelymail_domain.example.name
}

# Record order (stable): [0] ownership TXT, [1] MX, [2] SPF,
# [3-5] DKIM CNAMEs, [6] DMARC, [7] autoconfig, [8] autodiscover SRV.

resource "cloudflare_dns_record" "ownership" {
  zone_id = var.zone_id
  name    = "example.com"
  type    = "TXT"
  content = "\"${data.purelymail_dns_records.example.ownership_txt}\""
}

resource "cloudflare_dns_record" "mx" {
  zone_id  = var.zone_id
  name     = "example.com"
  type     = "MX"
  content  = data.purelymail_dns_records.example.records[1].content
  priority = data.purelymail_dns_records.example.records[1].priority
}
```

## Argument Reference

- `domain_name` — (Required) Domain name, e.g. `example.com`.

## Attribute Reference

- `id` — Same as `domain_name`.
- `ownership_txt` — Full TXT record value for ownership proof (also the first element of `records`).
- `records` — List of records, each with `type`, `name`, `content`, and `priority` (0 for non-MX/SRV records).
