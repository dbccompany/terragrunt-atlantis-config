package cmd

import (
	"bytes"

	log "github.com/sirupsen/logrus"
	"path/filepath"
	"strings"
	"testing"
)

// The library engine is being deprecated; a migration warning must fire when
// (and only when) it is the resolved engine.
func TestLibraryEngineDeprecationWarning(t *testing.T) {
	if err := resetForRun(); err != nil {
		t.Fatal(err)
	}
	defer func() { engine = engineLibrary }() // resetForRun default

	var buf bytes.Buffer
	origOut := log.StandardLogger().Out
	log.SetOutput(&buf)
	defer log.SetOutput(origOut)

	_, err := RunWithFlags("test_artifacts/deprecation.yaml", []string{
		"generate", "--engine", "library",
		"--output", "test_artifacts/deprecation.yaml",
		"--root", filepath.Join("..", "test_examples", "basic_module"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "deprecated: the embedded terragrunt library engine") {
		t.Fatalf("expected library-engine deprecation warning, got:\n%s", buf.String())
	}
}

// The CLI engine must not emit the deprecation warning.
func TestCLIEngineNoDeprecationWarning(t *testing.T) {
	terragruntCLIOrSkip(t)
	if err := resetForRun(); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	origOut := log.StandardLogger().Out
	log.SetOutput(&buf)
	defer log.SetOutput(origOut)

	_, err := RunWithFlags("test_artifacts/cli_deprecation.yaml", []string{
		"generate", "--engine", "cli",
		"--output", "test_artifacts/cli_deprecation.yaml",
		"--root", filepath.Join("..", "test_examples", "basic_module"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "deprecated: the embedded terragrunt library engine") {
		t.Fatalf("cli engine must not warn about library deprecation:\n%s", buf.String())
	}
}
