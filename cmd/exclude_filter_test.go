package cmd

import "testing"

// https://github.com/transcend-io/terragrunt-atlantis-config/issues/248
// --exclude subtracts directories (and subtrees) from the discovered set.
func TestExcludeSubtractsProjects(t *testing.T) {
	defer func() { excludePaths = nil }() // not covered by resetForRun
	runTest(t, "golden/exclude_dep.yaml", []string{
		"--exclude", "dep",
		"--root", "../test_examples_issues/depends_on_duplicate",
	})
}

// Exclude composes with --filter: keep only depender* then drop nested.
func TestExcludeComposesWithFilter(t *testing.T) {
	defer func() { excludePaths = nil }()
	if err := resetForRun(); err != nil {
		t.Fatal(err)
	}
	filename := "test_artifacts/exclude_compose.yaml"
	content, err := RunWithFlags(filename, []string{
		"generate",
		"--output", filename,
		"--filter", "../test_examples_issues/depends_on_duplicate",
		"--exclude", "app",
		"--root", "../test_examples_issues/depends_on_duplicate",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(content); len(got) == 0 {
		t.Fatal("no output")
	}
	// dep must survive, app must not
	if !containsDir(string(content), "dir: dep") || containsDir(string(content), "dir: app") {
		t.Fatalf("exclude+filter composition wrong:\n%s", content)
	}
}
