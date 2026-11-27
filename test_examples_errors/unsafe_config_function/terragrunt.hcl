locals {
  x = run_cmd("echo", "ok")
}

terraform {
  source = "."
}
