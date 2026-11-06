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
