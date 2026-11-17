terraform {
  source = "git::git@github.com:transcend-io/terraform-aws-fargate-container?ref=v0.0.4"
}

dependency "ws" {
  config_path = "../.."
}

inputs = {
  foo = dependency.ws.outputs.some_output
}
