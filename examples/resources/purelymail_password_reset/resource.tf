resource "purelymail_password_reset" "backup" {
  user_name   = purelymail_user.example.id
  type        = "email"
  target      = "recovery@example.net"
  description = "Backup email"
}
