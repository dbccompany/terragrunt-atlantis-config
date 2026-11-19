terraform {
  source = "git::git@github.com:transcend-io/terraform-aws-fargate-container?ref=v0.0.4"
}

dependency "parent" {
  config_path = ".."
}

inputs = {
  foo = dependency.parent.outputs.some_output
}
