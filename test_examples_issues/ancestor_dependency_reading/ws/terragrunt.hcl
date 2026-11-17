terraform {
  source = "git::git@github.com:transcend-io/terraform-aws-fargate-container?ref=v0.0.4"
}

locals {
  common = read_terragrunt_config("${get_terragrunt_dir()}/shared/common.hcl")
}
