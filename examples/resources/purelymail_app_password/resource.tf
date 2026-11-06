resource "purelymail_app_password" "laptop" {
  user_handle = purelymail_user.example.id
  name        = "laptop"
}
