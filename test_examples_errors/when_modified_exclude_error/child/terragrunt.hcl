terraform {
  source = "git::git@github.com:transcend-io/terraform-aws-fargate-container?ref=v0.0.4"
}

locals {
  tg_config = read_terragrunt_config(find_in_parent_folders("config.hcl"))

  atlantis_when_modified_exclude = [
    "_modules/**",
    "some_valid_glob",
    local.tg_config,
  ]
}