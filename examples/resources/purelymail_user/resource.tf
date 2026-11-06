resource "purelymail_user" "example" {
  user_name   = "hello"
  domain_name = purelymail_domain.example.name
  password    = var.mailbox_password
}
