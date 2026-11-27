package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestScrubbedEnv(t *testing.T) {
	t.Setenv("PATH", "/keep/me")
	t.Setenv("TERRAGRUNT_TFPATH", "/usr/local/bin/tofu")
	t.Setenv("ATLANTIS_GH_TOKEN", "canary-vcs")
	t.Setenv("ATLANTIS_GH_WEBHOOK_SECRET", "canary-webhook")
	t.Setenv("GITHUB_TOKEN", "canary-gh")
	t.Setenv("SSH_AUTH_SOCK", "/tmp/canary-agent")
	t.Setenv("AWS_ACCESS_KEY_ID", "kept-cloud-credential")

	env := scrubbedEnv()
	got := make(map[string]string, len(env))
	for _, kv := range env {
		key, value, _ := strings.Cut(kv, "=")
		got[key] = value
	}

	assert.Equal(t, "/keep/me", got["PATH"])
	assert.Equal(t, "/usr/local/bin/tofu", got["TERRAGRUNT_TFPATH"])
	assert.Equal(t, "kept-cloud-credential", got["AWS_ACCESS_KEY_ID"])
	for _, key := range []string{
		"ATLANTIS_GH_TOKEN", "ATLANTIS_GH_WEBHOOK_SECRET", "GITHUB_TOKEN", "SSH_AUTH_SOCK",
	} {
		assert.NotContains(t, got, key, "credential %q must be scrubbed", key)
	}
}

func TestGuardConfigTree(t *testing.T) {
	root := t.TempDir()
	writeFile := func(rel, content string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeFile("clean/terragrunt.hcl", `locals { x = "ok" }`)
	assert.NoError(t, guardConfigTree(root))

	writeFile("unsafe/terragrunt.hcl", `locals { x = run_cmd("sh", "-c", "true") }`)
	err := guardConfigTree(root)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "run_cmd")
	assert.Contains(t, err.Error(), "--allow-config-command-execution")

	// Nested blocks and attributes count too.
	nested := t.TempDir()
	nestedFile := filepath.Join(nested, "terragrunt.hcl")
	if err := os.WriteFile(nestedFile, []byte(`terraform {
  extra_arguments "x" {
    arguments = [sops_decrypt_file("secrets.yaml")]
  }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	err = guardConfigTree(nested)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sops_decrypt_file")
}

func TestGuardConfigTreeIgnoresHiddenDirs(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".terragrunt-cache", "terragrunt.hcl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`locals { x = run_cmd("sh", "-c", "true") }`), 0o644); err != nil {
		t.Fatal(err)
	}
	assert.NoError(t, guardConfigTree(root))
}

// The library engine must refuse to evaluate command-executing config
// functions rather than executing them.
func TestLibraryEngineRefusesUnsafeConfigFunction(t *testing.T) {
	err := resetForRun()
	if err != nil {
		t.Fatal(err)
	}

	// The evaluation failure is reported through the terragrunt diagnostics
	// logger; capture it so the assertion is on the real message.
	var buf strings.Builder
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	rootCmd.SetArgs([]string{
		"generate",
		"--engine", "library",
		"--root", filepath.Join("..", "test_examples_errors", "unsafe_config_function"),
	})
	err = rootCmd.Execute()
	if err == nil {
		t.Fatal("expected generation to fail on a config that calls run_cmd")
	}
	assert.Contains(t, buf.String(), "not available during config generation")
}

// The CLI engine refuses to invoke terragrunt discovery when a config under
// the root references a command-executing function, unless the escape hatch
// is passed.
func TestCLIEngineRefusesUnsafeConfigFunction(t *testing.T) {
	terragruntCLIOrSkip(t)

	err := resetForRun()
	if err != nil {
		t.Fatal(err)
	}

	root := filepath.Join("..", "test_examples_errors", "unsafe_config_function")
	rootCmd.SetArgs([]string{
		"generate",
		"--engine", "cli",
		"--root", root,
	})
	err = rootCmd.Execute()
	if err == nil {
		t.Fatal("expected generation to refuse a config that calls run_cmd")
	}
	assert.Contains(t, err.Error(), "refuses to run")

	rootCmd.SetArgs([]string{
		"generate",
		"--engine", "cli",
		"--root", root,
		"--allow-config-command-execution",
	})
	assert.NoError(t, rootCmd.Execute())
}
