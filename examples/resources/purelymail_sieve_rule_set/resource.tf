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
