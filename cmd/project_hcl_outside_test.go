package cmd

import "testing"

// Regression for https://github.com/transcend-io/terragrunt-atlantis-config/issues/421
// (project-hcl-files dropped dependencies whose path shared a prefix with the
// working dir, e.g. "app-secrets" vs "app").
func TestProjectHclFilesIncludeSiblingShared(t *testing.T) {
	runTest(t, "golden/project_hcl_outside.yaml", []string{
		"--project-hcl-files", "marker.hcl",
		"--root", "../test_examples_issues/project_hcl_outside",
	})
}
