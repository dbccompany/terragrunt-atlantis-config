terraform {
  source = "git::git@github.com:transcend-io/terraform-aws-fargate-container?ref=v0.0.4"
}

exclude {
  if      = true
  actions = ["all_except_output"]
}

locals {
  common = read_terragrunt_config("${get_terragrunt_dir()}/shared/common.hcl")
}
