resource "purelymail_routing_rule" "catchall" {
  domain_name      = purelymail_domain.example.name
  prefix           = false
  match_user       = "*"
  target_addresses = ["hello@example.com"]
  catchall         = true
}
