package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// configExecFunctionNames are the terragrunt config functions that execute
// external programs. The library engine stubs them during evaluation; the
// CLI engine cannot (evaluation happens inside the terragrunt binary), so it
// refuses to invoke terragrunt discovery when any config under the root
// references them. The --allow-config-command-execution escape hatch restores
// the previous behaviour for repositories that need it.
var configExecFunctionNames = map[string]bool{
	"run_cmd":           true,
	"sops_decrypt_file": true,
}

var errUnsafeConfigFunction = errors.New("unsafe config function")

// guardConfigTree scans every .hcl file under root and fails generation when
// one of them references a command-executing config function. Unreadable or
// unparseable files are left to terragrunt to report on.
func guardConfigTree(root string) error {
	offender := ""
	called := ""
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			// Skip VCS, terraform and tooling scratch dirs.
			if name != "." && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".hcl") {
			return nil
		}
		raw, err := readFileAsString(path)
		if err != nil {
			return nil
		}
		file, diags := hclsyntax.ParseConfig([]byte(raw), path, hcl.InitialPos)
		if diags.HasErrors() {
			return nil
		}
		if called = hclBodyExecutesCommands(file.Body.(*hclsyntax.Body)); called != "" {
			offender = path
			return errUnsafeConfigFunction
		}
		return nil
	})
	if errors.Is(err, errUnsafeConfigFunction) {
		return fmt.Errorf("config file %q calls %q, which generation refuses to run; pass --allow-config-command-execution to override", offender, called)
	}
	return err
}

// hclBodyExecutesCommands returns the name of the first command-executing
// config function called anywhere in the body (blocks and attributes
// included), or "" when the body is clean.
func hclBodyExecutesCommands(body *hclsyntax.Body) string {
	for _, attr := range body.Attributes {
		if name := exprExecutesCommands(attr.Expr); name != "" {
			return name
		}
	}
	for _, block := range body.Blocks {
		if name := hclBodyExecutesCommands(block.Body); name != "" {
			return name
		}
	}
	return ""
}

func exprExecutesCommands(expr hclsyntax.Expression) string {
	finder := &commandFunctionFinder{names: configExecFunctionNames}
	hclsyntax.Walk(expr, finder)
	return finder.called
}

// commandFunctionFinder records whether an HCL syntax tree calls any of the
// command-executing config functions.
type commandFunctionFinder struct {
	names  map[string]bool
	called string
}

func (f *commandFunctionFinder) Enter(node hclsyntax.Node) hcl.Diagnostics {
	if call, ok := node.(*hclsyntax.FunctionCallExpr); ok && f.names[call.Name] && f.called == "" {
		f.called = call.Name
	}
	return nil
}

func (f *commandFunctionFinder) Exit(node hclsyntax.Node) hcl.Diagnostics { return nil }

// scrubbedEnv returns the process environment without credential-bearing
// variables, for subprocesses that evaluate repository config files. The
// evaluation itself can execute code from the repository; keeping
// credentials out of that subprocess limits what such code can reach.
func scrubbedEnv() []string {
	scrub := func(key string) bool {
		upper := strings.ToUpper(key)
		if strings.HasPrefix(upper, "ATLANTIS_") {
			return true
		}
		switch upper {
		case "GITHUB_TOKEN", "GH_TOKEN", "GH_ENTERPRISE_TOKEN",
			"GITLAB_TOKEN", "GITLAB_CI_TOKEN", "BITBUCKET_TOKEN",
			"AZURE_DEVOPS_TOKEN", "SSH_AUTH_SOCK", "SSH_AGENT_PID",
			"GIT_ASKPASS", "NETRC":
			return true
		}
		return false
	}

	environ := os.Environ()
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		key, _, _ := strings.Cut(kv, "=")
		if !scrub(key) {
			out = append(out, kv)
		}
	}
	return out
}
