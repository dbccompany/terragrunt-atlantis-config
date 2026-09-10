include "shared" {
  path = "../app-secrets/shared.hcl"
}

terraform {
  source = "git::git@github.com:example-corp/infra-modules.git//app?ref=v1.0.0"
}
